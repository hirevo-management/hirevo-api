package reports

import (
	"hirevo/internal/handlers"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// RegisterHooks update company and user reports
func RegisterHooks(app *pocketbase.PocketBase) {
	updateCompanyReportOnJobChange(app)
	updateCompanyReportOnInvoiceChange(app)
	updateUserReportOnJobMemberChange(app)
}

// Job observer (create/update) -> company_reports
func updateCompanyReportOnJobChange(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("jobs").BindFunc(func(e *core.RecordEvent) error {
		companyID := e.Record.GetString("companyID")
		return updateCompanyReport(app, companyID)
	})

	app.OnRecordAfterUpdateSuccess("jobs").BindFunc(func(e *core.RecordEvent) error {
		companyID := e.Record.GetString("companyID")
		return updateCompanyReport(app, companyID)
	})
}

// Invoice observer (create/update) -> company_reports
func updateCompanyReportOnInvoiceChange(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("invoices").BindFunc(func(e *core.RecordEvent) error {
		companyID := e.Record.GetString("companyID")
		return updateCompanyReport(app, companyID)
	})

	app.OnRecordAfterUpdateSuccess("invoices").BindFunc(func(e *core.RecordEvent) error {
		companyID := e.Record.GetString("companyID")
		return updateCompanyReport(app, companyID)
	})
}

// Job members observer (create/update) -> user_reports
func updateUserReportOnJobMemberChange(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("job_members").BindFunc(func(e *core.RecordEvent) error {
		userID := e.Record.GetString("userID")
		return updateUserReport(app, userID)
	})

	app.OnRecordAfterUpdateSuccess("job_members").BindFunc(func(e *core.RecordEvent) error {
		userID := e.Record.GetString("userID")
		return updateUserReport(app, userID)
	})
}

func updateCompanyReport(app *pocketbase.PocketBase, companyID string) error {
	collection, err := app.FindCollectionByNameOrId("company_reports")
	if err != nil {
		handlers.LogError(err, "Failed to find company_reports collection -> CompanyID received", "companyID", companyID)
		return handlers.InternalServerError("Failed to find company_reports collection -> CompanyID received", err, "companyID", companyID)
	}

	// Fetch or create company report
	report, err := app.FindFirstRecordByFilter("company_reports", "companyID = {:companyID}", dbx.Params{
		"companyID": companyID,
	})
	if err != nil {
		handlers.LogInfo("No existing company report found, creating new", "companyID", companyID)
		report = core.NewRecord(collection)
		report.Set("companyID", companyID)
	}

	// metrics
	jobs, err := app.FindRecordsByFilter("jobs", "companyID = {:companyID}", "-created", 0, 0, dbx.Params{
		"companyID": companyID,
	})
	if err != nil {
		return err
	}
	totalJobs := len(jobs)
	activeJobs := 0
	completedJobs := 0
	for _, job := range jobs {
		switch job.GetString("status") {
		case "HIRING", "READY":
			activeJobs++
		case "COMPLETED":
			completedJobs++
		}
	}

	members, err := app.FindRecordsByFilter("company_members", "companyID = {:companyID} && status = 'ACTIVE'", "-created", 0, 0, dbx.Params{
		"companyID": companyID,
	})
	if err != nil {
		return err
	}
	totalWorkers := len(members)

	invoices, err := app.FindRecordsByFilter("invoices", "companyID = {:companyID}", "-created", 0, 0, dbx.Params{
		"companyID": companyID,
	})
	if err != nil {
		handlers.LogError(err, "Failed to fetch company invoices report", "companyID", companyID)
		return handlers.InternalServerError("Failed to fetch company invoices report", err, "companyID", companyID)
	}
	totalInvoices := len(invoices)
	paidInvoices := 0
	totalRevenue := 0.0
	for _, inv := range invoices {
		if inv.GetString("status") == "PAID" {
			paidInvoices++
			metadata := inv.Get("metadata").(map[string]interface{})
			if total, ok := metadata["Total"].(float64); ok {
				totalRevenue += total
			}
		}
	}

	// update company_reports
	report.Set("totalJobs", totalJobs)
	report.Set("activeJobs", activeJobs)
	report.Set("completedJobs", completedJobs)
	report.Set("totalWorkers", totalWorkers)
	report.Set("totalInvoices", totalInvoices)
	report.Set("paidInvoices", paidInvoices)
	report.Set("totalRevenue", totalRevenue)

	if err := app.SaveNoValidate(report); err != nil {
		handlers.LogError(err, "Failed to save company report", "companyID", companyID)
		return handlers.InternalServerError("Failed to save company report", err, "companyID", companyID)
	}

	handlers.LogInfo("Company report updated successfully", "companyID", companyID, "totalJobs", totalJobs, "activeJobs", activeJobs, "completedJobs", completedJobs, "totalWorkers", totalWorkers, "totalInvoices", totalInvoices, "paidInvoices", paidInvoices, "totalRevenue", totalRevenue)
	return nil
}

func updateUserReport(app *pocketbase.PocketBase, userID string) error {
	collection, err := app.FindCollectionByNameOrId("user_reports")
	if err != nil {
		handlers.LogError(err, "Failed to find user_reports collection", "collection", "user_reports")
		return handlers.InternalServerError("Failed to find user_reports collection", err, "user_reports", "user_reports")
	}

	// Fetch or create user reports
	report, err := app.FindFirstRecordByFilter("user_reports", "userID = {:userID}", dbx.Params{
		"userID": userID,
	})
	if err != nil {
		handlers.LogInfo("No existing user report found, creating new", "userID", userID)
		report = core.NewRecord(collection)
		report.Set("userID", userID)
	}

	// metrics
	jobMembers, err := app.FindRecordsByFilter("job_members", "userID = {:userID}", "-created", 0, 0, dbx.Params{
		"userID": userID,
	})
	if err != nil {
		handlers.LogError(err, "Failed to fetch job_members", "userID", userID)
		return handlers.InternalServerError("Failed to fetch job_members during generate reports", err, "userID", userID)
	}
	totalJobs := len(jobMembers)
	hiredJobs := 0
	totalHours := 0.0
	totalEarnings := 0.0
	for _, jm := range jobMembers {
		if jm.GetString("status") == "HIRED" {
			hiredJobs++
			jobID := jm.GetString("jobID")
			job, err := app.FindRecordById("jobs", jobID)
			if err != nil {
				handlers.LogWarn("Job not found for job_member", "jobID", jobID)
				continue
			}
			rateIds, ok := job.Get("rates").([]interface{})
			if !ok || len(rateIds) == 0 {
				handlers.LogWarn("No rates found for job", "jobID", jobID)
				continue
			}
			// Convert IDs -> strings
			var rateIdStrings []string
			for _, id := range rateIds {
				if idStr, ok := id.(string); ok {
					rateIdStrings = append(rateIdStrings, idStr)
				}
			}
			if len(rateIdStrings) == 0 {
				continue
			}
			rates, err := app.FindRecordsByFilter("job_rates", "id IN ({:rateIds})", "-created", 0, 0, dbx.Params{
				"rateIds": rateIdStrings,
			})
			if err != nil {
				handlers.LogError(err, "Failed to fetch job rates", "jobID", jobID)
				continue
			}
			for _, rate := range rates {
				// Convert date fields (strings) -> time.Time
				startStr := rate.GetString("startTime")
				endStr := rate.GetString("endTime")
				start, err := time.Parse(time.RFC3339, startStr)
				if err != nil {
					handlers.LogWarn("Invalid startTime format", "startTime", startStr)
					continue
				}
				end, err := time.Parse(time.RFC3339, endStr)
				if err != nil {
					handlers.LogWarn("Invalid endTime format", "endTime", endStr)
					continue
				}
				hours := end.Sub(start).Hours()
				totalHours += hours
				totalEarnings += hours * rate.GetFloat("rateValue")
			}
		}
	}

	companies, err := app.FindRecordsByFilter("company_members", "userID = {:userID} && status = 'ACTIVE'", "-created", 0, 0, dbx.Params{
		"userID": userID,
	})
	if err != nil {
		handlers.LogError(err, "Failed to fetch company_members", "userID", userID)
		return handlers.InternalServerError("Failed to fetch company_members during generate reports", err, "userID", userID)
	}
	activeCompanies := len(companies)

	// Update user_reports
	report.Set("totalJobs", totalJobs)
	report.Set("hiredJobs", hiredJobs)
	report.Set("totalHours", totalHours)
	report.Set("totalEarnings", totalEarnings)
	report.Set("activeCompanies", activeCompanies)

	if err := app.SaveNoValidate(report); err != nil {
		handlers.LogError(err, "Failed while saving user report", "userID", userID)
		return handlers.InternalServerError("Failed while saving user report", err, "userID", userID)
	}
	handlers.LogInfo("User report updated successfully", "userID", userID, "totalJobs", totalJobs, "hiredJobs", hiredJobs, "totalHours", totalHours, "totalEarnings", totalEarnings)
	return nil
}
