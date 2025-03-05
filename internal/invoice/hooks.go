package invoice

import (
	"encoding/json"
	"fmt"
	"hirevo/internal/handlers"
	pdfgenerator "hirevo/services/pdf"
	"io"
	"net/http"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// RegisterHooks fetch, validate and generate invoices
func RegisterHooks(app *pocketbase.PocketBase) {
	onGenerateInvoiceRequest(app)
}

func onGenerateInvoiceRequest(app *pocketbase.PocketBase) {
	app.OnRecordCreate("invoices").BindFunc(func(e *core.RecordEvent) error {
		// Extract metadata attribute
		metadataRaw := e.Record.Get("metadata")
		content, err := validateBody(metadataRaw)
		if err != nil {
			handlers.LogError(err, "Failed while convert invoice attributes", content)
			return handlers.BadRequestError("Failed while convert invoice attributes", err)
		}
		companyIDRaw := e.Record.Get("companyID")
		companyID, ok := companyIDRaw.(string)
		if !ok || companyID == "" {
			handlers.LogError(err, "Failed to process invoice creation due to invalid companyID", "companyIDRaw", companyIDRaw)
			return handlers.BadRequestError("Missing or invalid 'companyID'", companyIDRaw)
		}

		userIDRaw := e.Record.Get("userID")
		userID, ok := userIDRaw.(string)
		if !ok || userID == "" {
			handlers.LogError(err, "Failed to process invoice creation due to invalid userID", "userIDRaw", userIDRaw)
			return handlers.BadRequestError("Missing or invalid 'userID'", userIDRaw)
		}
		fullMetadata, err := buildFullMetadata(app, companyID, userID, content)
		pdfData := pdfgenerator.PDFData{
			Title:       fullMetadata["Title"].(string),
			HeaderImage: fullMetadata["HeaderImage"].([]byte),
			Header:      fullMetadata["Header"].(string),
			Content:     fullMetadata["Content"].(map[string]string),
			Footer:      fullMetadata["Footer"].(string),
		}

		// Generate PDF
		pdfBytes, err := pdfgenerator.GeneratePDFBytes(pdfData)
		if err != nil {
			handlers.LogError(err, "Failed while generate PDF invoice")
			return handlers.InternalServerError("Failed while generate PDF invoice", err)
		}

		//Create file from PDF bytes
		file, err := filesystem.NewFileFromBytes(pdfBytes, "invoice.pdf")
		if err != nil {
			handlers.LogError(err, "Failed while generate PDF from bytes")
			return handlers.InternalServerError("Failed while generate PDF invoice", err)
		}

		e.Record.Set("metadata", fullMetadata)
		e.Record.Set("status", "PENDING")
		e.Record.Set("doc", file)

		handlers.LogInfo("Create PDF invoice successfully", "companyID", companyID, "userID", userID)
		return e.Next()
	})
}

func validateBody(metadataRaw any) (map[string]string, error) {
	if metadataRaw == nil {
		handlers.LogWarn("Missing metadata", "metadata", metadataRaw)
		err := handlers.BadRequestError("Missing metadata field", "", validation.NewError(
			"invalid_metadata",
			"The 'metadata' field is required",
		))
		return nil, err
	}

	metadataBytes, err := json.Marshal(metadataRaw)
	if err != nil {
		handlers.LogError(err, "Failed to format JSON marshal metadata attribute 'json.Marshal(metadataRaw)'", "metadata", metadataRaw)
		err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
			"invalid_metadata",
			"Invalid 'metadata' field",
		))
		return nil, err
	}

	type MetadataRequest struct {
		Content map[string]interface{} `json:"Content"`
	}
	var metadataReq MetadataRequest
	if err := json.Unmarshal(metadataBytes, &metadataReq); err != nil {
		handlers.LogError(err, "Failed to format JSON marshal metadata attribute 'json.Unmarshal(metadataBytes, &metadataReq)'", "metadataBytes", metadataBytes)
		err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
			"invalid_metadata",
			"Invalid 'metadata' field",
		))
		return nil, err
	}
	if metadataReq.Content == nil {
		handlers.LogError(err, "Failed to found Content field on metadata request", "Content", metadataReq.Content)
		err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
			"invalid_metadata",
			"Failed to found Content field on metadata request",
		))
		return nil, err
	}
	// Validate map[string]string
	content := make(map[string]string)
	for key, value := range metadataReq.Content {
		if strValue, ok := value.(string); ok {
			content[key] = strValue
		} else {
			handlers.LogWarn("Invalid Content", "Key", key, "Value", value)
			err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
				"invalid_metadata",
				fmt.Sprintf("Inalid Content: Key '%s' is %T type, but the function needs string", key, value),
			))
			return nil, err
		}
	}

	return content, nil
}

func buildFullMetadata(app *pocketbase.PocketBase, companyID string, userID string, content map[string]string) (map[string]interface{}, error) {
	//Fetch company data
	company, err := app.FindRecordById("companies", companyID)
	if err != nil {
		handlers.LogError(err, "Not found record id during build full metadata PDF invoice", "CompanyID", companyID)
		err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
			"invalid_metadata",
			fmt.Sprintf("Not found company while creation Invoice with company id '%s'", companyID),
		))
		return nil, err
	}

	//Fetch user data
	users, err := app.FindRecordById("users", userID)
	if err != nil {
		handlers.LogError(err, "Not found record id during build full metadata PDF invoice dor userID", "userID", userID)
		err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
			"invalid_metadata",
			fmt.Sprintf("Not found user while creation Invoice with user id '%s'", userID),
		))
		return nil, err
	}

	userName := users.GetString("name")

	name := company.GetString("name")
	abn := company.GetString("abn")
	phone := company.GetString("phone")
	email := company.GetString("email")
	website := company.GetString("website")
	header := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", name, abn, phone, email, website)

	// Fetch company logo if exists
	var logoBase64 []byte
	if logoFile := company.GetString("logo"); logoFile != "" {

		//TODO: Uses environment variables
		baseURL := "http://localhost:8090"
		logoUrl := fmt.Sprintf("%s/api/files/companies/%s/%s", baseURL, companyID, logoFile)
		logo, err := fetchLogoBase64(logoUrl)
		if err != nil {
			handlers.LogError(err, "Failed fetch company logo while generating invoice", "logo", logo)
			err := handlers.BadRequestError("Invalid metadata field", "", validation.NewError(
				"invalid_invoice",
				fmt.Sprintf("Failed fetch company logo while generating invoice"),
			))
			return nil, err
		}
		logoBase64 = logo
	}

	//Update metadata JSON
	completeMap := make(map[string]interface{})
	completeMap["Title"] = strings.ToUpper(userName)
	completeMap["HeaderImage"] = logoBase64
	completeMap["Header"] = header
	completeMap["Content"] = content
	completeMap["Footer"] = ""

	return completeMap, nil
}

func fetchLogoBase64(logoUrl string) ([]byte, error) {
	resp, err := http.Get(logoUrl)
	if err != nil {
		handlers.LogError(err, "Failed download company logo while generating invoice", "logoUrl", logoUrl)
		err := handlers.BadRequestError("Invalid base url", "", validation.NewError(
			"invalid_url",
			fmt.Sprintf("Failed download company logo while generating invoice"),
		))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		handlers.LogError(err, "Response code is invalid generating invoice", "Status code", resp.StatusCode)
		err := handlers.BadRequestError("Invalid base url", "", validation.NewError(
			"invalid_url",
			fmt.Sprintf("Failed while generating invoice"),
		))
		return nil, err
	}

	logoBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		handlers.LogError(err, "Failed read logo bytes while generating invoice")
		err := handlers.BadRequestError("Invalid company logo", "", validation.NewError(
			"invalid_url",
			fmt.Sprintf("Failed while generating invoice"),
		))
		return nil, err
	}

	return logoBytes, nil
}
