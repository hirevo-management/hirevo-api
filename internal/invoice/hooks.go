package invoice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	pdfgenerator "hirevo/services/pdf"
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
			return fmt.Errorf("falha ao converter: %v", err)
		}
		companyID := e.Record.Get("companyID").(string)
		userID := e.Record.Get("userID").(string)

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
			return fmt.Errorf("falha ao gerar PDF: %v", err)
		}

		//Create file from PDF bytes
		file, err := filesystem.NewFileFromBytes(pdfBytes, "invoice.pdf")
		if err != nil {
			return fmt.Errorf("falha ao criar arquivo a partir dos bytes: %v", err)
		}

		e.Record.Set("metadata", fullMetadata)
		e.Record.Set("status", "PENDING")
		e.Record.Set("doc", file)

		return e.Next()
	})
}

func validateBody(metadataRaw any) (map[string]string, error) {
	if metadataRaw == nil {
		return nil, fmt.Errorf("campo metadata não encontrado no request")
	}

	metadataBytes, err := json.Marshal(metadataRaw)
	if err != nil {
		return nil, fmt.Errorf("falha ao marshall metadata: %v", err)
	}

	type MetadataRequest struct {
		Content map[string]interface{} `json:"Content"`
	}
	var metadataReq MetadataRequest
	if err := json.Unmarshal(metadataBytes, &metadataReq); err != nil {
		return nil, fmt.Errorf("falha ao unmarshal metadata: %v", err)

	}
	if metadataReq.Content == nil {
		return nil, fmt.Errorf("campo Content não encontrado no metadata")
	}
	// Validate map[string]string
	content := make(map[string]string)
	for key, value := range metadataReq.Content {
		if strValue, ok := value.(string); ok {
			content[key] = strValue
		} else {
			return nil, fmt.Errorf("valor inválido no Content: chave '%s' contém tipo %T, esperado string", key, value)
		}
	}

	return content, nil
}

func buildFullMetadata(app *pocketbase.PocketBase, companyID string, userID string, content map[string]string) (map[string]interface{}, error) {
	//Fetch company data
	company, err := app.FindRecordById("companies", companyID)
	if err != nil {
		return nil, err
	}

	//Fetch user data
	users, err := app.FindRecordById("users", userID)
	if err != nil {
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
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8090"
		} else if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			return nil, fmt.Errorf("BASE_URL inválida: %s", baseURL)
		}

		logoUrl := fmt.Sprintf("%s/api/files/companies/%s/%s", baseURL, companyID, logoFile)
		logo, err := fetchLogoBase64(logoUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch logo: %w", err)
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
		return nil, fmt.Errorf("failed to download logo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	logoBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read logo bytes: %w", err)
	}

	return logoBytes, nil
}
