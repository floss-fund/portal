package main

import (
	"log"
	"os"
	"text/template"

	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/crawl"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

var (
	buildString   = "unknown"
	versionString = "unknown"
)

type Consts struct {
	RootURL      string `json:"app.root_url"`
	ManifestURI  string `json:"app.manifest_path"`
	WellKnownURI string `json:"app.wellknown_path"`
}

// App contains the "global" components that are passed around, especially through HTTP handlers.
type App struct {
	consts  Consts
	siteTpl *template.Template
	core    *core.Core
	crawl   *crawl.Crawl

	db *sqlx.DB
	fs stuffbin.FileSystem
	lo *log.Logger
}

var (
	lo = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	ko = koanf.New(".")
)

func main() {
	initConfig()

	// Connect to the DB.
	db := initDB(ko.MustString("db.host"),
		ko.MustInt("db.port"),
		ko.MustString("db.user"),
		ko.MustString("db.password"),
		ko.MustString("db.db"),
	)
	defer db.Close()

	// Initialize the app context that's passed around.
	app := &App{
		consts: initConstants(ko),
		db:     db,
		fs:     initFS(),
		lo:     lo,
	}

	// Install or upgrade schema.
	if ko.Bool("install") {
		installSchema(migList[len(migList)-1].version, app, !ko.Bool("yes"))
		return
	}
	if ko.Bool("upgrade") {
		upgrade(db, app.fs, !ko.Bool("yes"))
		os.Exit(0)
	}

	// Before the queries are prepared, see if there are pending upgrades.
	checkUpgrade(db)

	// Initialize queries and data handler.
	app.core = initCore(app.fs, db, ko)
	app.crawl = initCrawl(initSchema(ko), app.core, ko)

	// Run the crawl mode.
	if ko.String("mode") == "crawl" {
		app.crawl.Crawl()
		return
	}

	// Initialize the echo HTTP server.
	srv := initHTTPServer(app, ko)

	lo.Printf("starting server on %s", ko.MustString("app.address"))
	if err := srv.Start(ko.MustString("app.address")); err != nil {
		lo.Fatalf("error starting HTTP server: %v", err)
	}
}
