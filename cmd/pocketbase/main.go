package main

import (
	"hirevo/internal/company"
	"hirevo/internal/handlers"
	"hirevo/internal/invoice"
	"hirevo/internal/reports"

	"github.com/pocketbase/pocketbase"
)

func main() {
	app := pocketbase.New()
	initializeHandlers(app)
	initializeHooks(app)

	if err := app.Start(); err != nil {
		handlers.LogError(err, "Failed to start app")
	}
}

func initializeHandlers(app *pocketbase.PocketBase) {
	handlers.InitLogger(app)
	handlers.InitErrorHandler(app)
}

func initializeHooks(app *pocketbase.PocketBase) {
	company.RegisterHooks(app)
	invoice.RegisterHooks(app)
	reports.RegisterHooks(app)
}
