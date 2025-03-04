package main

import (
	"hirevo/internal/reports"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"hirevo/internal/company"
	"hirevo/internal/invoice"
)

func main() {
	app := pocketbase.New()
	openDatabaseConnection(app)
	company.RegisterHooks(app)
	invoice.RegisterHooks(app)
	reports.RegisterHooks(app)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func openDatabaseConnection(app *pocketbase.PocketBase) {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found. Using default environment")
	}

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))
		return se.Next()
	})
}
