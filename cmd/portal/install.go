package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
)

func install(ver string, prompt bool, db, search bool, app *App, ko *koanf.Koanf) {
	if prompt {
		fmt.Println("")
		fmt.Println("** first time installation **")
		fmt.Printf("** IMPORTANT: This will wipe existing schema and data in db=%v, search=%v**", db, search)
		fmt.Println("")

		if prompt {
			var ok string
			fmt.Print("continue (y/n)?  ")
			if _, err := fmt.Scanf("%s", &ok); err != nil {
				fmt.Printf("error reading value from terminal: %v", err)
				os.Exit(1)
			}
			if strings.ToLower(ok) != "y" {
				fmt.Println("install cancelled.")
				return
			}
		}
	}

	if db {
		installDB(ver, app)
	}
	if search {
		installSearch(app, ko)
	}

	app.lo.Println("done")
}

func installDB(ver string, app *App) {
	q, err := app.fs.Read("/schema.sql")
	if err != nil {
		app.lo.Fatal(err.Error())
		return
	}

	if _, err := app.db.Exec(string(q)); err != nil {
		app.lo.Fatal(err.Error())
		return
	}

	// Insert the current migration version.
	if err := recordMigrationVersion(ver, app.db); err != nil {
		app.lo.Fatal(err)
	}

	app.lo.Println("installed Postgres schema")

}
func installSearch(app *App, ko *koanf.Koanf) {
	// Install Typesense schema.
	app.lo.Println("installing Typesense schema")

	s := initSearch(ko)
	if err := s.InitSchema(); err != nil {
		app.lo.Fatal(err)
	}

	app.lo.Println("installed typesense schema")
}

// recordMigrationVersion inserts the given version (of DB migration) into the
// `migrations` array in the settings table.
func recordMigrationVersion(ver string, db *sqlx.DB) error {
	_, err := db.Exec(fmt.Sprintf(`INSERT INTO settings (key, value)
	VALUES('migrations', '["%s"]'::JSONB)
	ON CONFLICT (key) DO UPDATE SET value = settings.value || EXCLUDED.value`, ver))
	return err
}
