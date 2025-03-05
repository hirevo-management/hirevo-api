package company

import (
	"encoding/json"
	"hirevo/internal/handlers"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// RegisterHooks Used for hooks related to company collection
func RegisterHooks(app *pocketbase.PocketBase) {
	onCreateCompanyRequest(app)
	onCreateCompanySuccess(app)
	onValidateCreateCompanyRequest(app)
}

func onCreateCompanyRequest(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("companies").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			handlers.LogError(err, "Error while getting RequestInfo")
			return handlers.BadRequestError("Failed to get request info", err)
		}
		if info.Auth == nil || info.Auth.Id == "" {
			handlers.LogWarn("No authenticated user for company creation", info)
			return handlers.ForbiddenError("No authenticated user for company creation", err)
		}

		userId := info.Auth.Id
		e.Record.Set("createdBy", userId)
		handlers.LogInfo("Company creation request", "userId", userId)
		return e.Next()
	})
}

func onCreateCompanySuccess(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("companies").BindFunc(func(e *core.RecordEvent) error {
		userId := e.Record.GetString("createdBy")
		companyId := e.Record.Id

		if userId == "" || companyId == "" {
			handlers.LogWarn("Missing userId or companyId after company creation", "userId", userId, "companyId", companyId)
			return e.Next()
		}

		collection, err := app.FindCollectionByNameOrId("company_members")
		if err != nil {
			handlers.LogError(err, "Failed to find company_members collection")
			return err
		}

		record := core.NewRecord(collection)
		record.Set("userID", userId)
		record.Set("companyID", companyId)
		record.Set("role", "OWNER")
		record.Set("status", "ACTIVE")
		err = app.Save(record)
		if err != nil {
			handlers.LogError(err, "Failed to save company member", "userId", userId, "companyId", companyId)
			return handlers.InternalServerError("Has error occurred during create company", err)
		}
		handlers.LogInfo("Company member created as OWNER", "userId", userId, "companyId", companyId)
		return e.Next()
	})
}

func onValidateCreateCompanyRequest(app *pocketbase.PocketBase) {
	app.OnRecordCreate("companies").BindFunc(func(e *core.RecordEvent) error {
		// Retrieve the "address" field
		addressRaw := e.Record.Get("address")
		jsonRaw, ok := addressRaw.(types.JSONRaw)

		if !ok {
			handlers.LogWarn("Failed to get JSON address", "address", addressRaw)
			return handlers.BadRequestError("Failed to get JSON address", "", validation.NewError(
				"invalid_address",
				"The 'address' field must be a JSON object with the required subfields.",
			))
		}

		// Unmarshal into a Go map
		var addressData map[string]any
		if err := json.Unmarshal(jsonRaw, &addressData); err != nil {
			handlers.LogError(err, "Address field validation failed", &addressData)
			return handlers.BadRequestError("Invalid JSON structure for 'address'", err, validation.NewError(
				"invalid_json",
				"Failed to parse the 'address' JSON",
			))
		}

		// Check required subfields
		requiredFields := []string{"street", "suburb", "state", "postcode", "country", "latitude", "longitude"}
		missing := []string{}
		for _, field := range requiredFields {
			if _, exists := addressData[field]; !exists {
				missing = append(missing, field)
			}
		}
		if len(missing) > 0 {
			handlers.LogWarn("Address validation failed", "missingFields", missing)
			return handlers.BadRequestError("Missing required subfields in 'address'", "", validation.NewError(
				"missing_subfields",
				"Required fields are missing: "+strings.Join(missing, ", "),
			))
		}

		lat, latOk := addressData["latitude"].(float64)
		lon, lonOk := addressData["longitude"].(float64)

		// Validate lat/lon -> Both must be decimal, lat range [-90,90] lon range [-190,180]
		if !latOk || !lonOk || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			handlers.LogWarn("Invalid coordinates in address", "latitude", lat, "longitude", lon)
			return handlers.BadRequestError("Missing required subfields in 'address'", "", validation.NewError(
				"invalid_coords",
				"Latitude must be between -90 and 90, and longitude between -180 and 180",
			))
		}
		handlers.LogInfo("Address validation successful", addressData)
		return e.Next()
	})
}
