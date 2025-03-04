package reports

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"time"
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

// Job memners observer (create/update) -> user_reports
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
		return err
	}

	// Fetch or create company report
	report, err := app.FindFirstRecordByFilter("company_reports", "companyID = {:companyID}", dbx.Params{
		"companyID": companyID,
	})
	if err != nil {
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
		return err
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

	return app.SaveNoValidate(report)
}

func updateUserReport(app *pocketbase.PocketBase, userID string) error {
	collection, err := app.FindCollectionByNameOrId("user_reports")
	if err != nil {
		return err
	}

	// Fetch or create user reports
	report, err := app.FindFirstRecordByFilter("user_reports", "userID = {:userID}", dbx.Params{
		"userID": userID,
	})
	if err != nil {
		report = core.NewRecord(collection)
		report.Set("userID", userID)
	}

	// metrics
	jobMembers, err := app.FindRecordsByFilter("job_members", "userID = {:userID}", "-created", 0, 0, dbx.Params{
		"userID": userID,
	})
	if err != nil {
		return err
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
				continue
			}
			rateIds, ok := job.Get("rates").([]interface{})
			if !ok || len(rateIds) == 0 {
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
				continue
			}
			for _, rate := range rates {
				// Convert date fields (strings) -> time.Time
				startStr := rate.GetString("startTime")
				endStr := rate.GetString("endTime")
				start, err := time.Parse(time.RFC3339, startStr)
				if err != nil {
					continue
				}
				end, err := time.Parse(time.RFC3339, endStr)
				if err != nil {
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
		return err
	}
	activeCompanies := len(companies)

	// Update user_reports
	report.Set("totalJobs", totalJobs)
	report.Set("hiredJobs", hiredJobs)
	report.Set("totalHours", totalHours)
	report.Set("totalEarnings", totalEarnings)
	report.Set("activeCompanies", activeCompanies)

	return app.SaveNoValidate(report)
}
