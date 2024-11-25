package main

import (
	"log"
	"os"
	"text/template"

	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/crawl"
	"github.com/floss-fund/portal/internal/search"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/paginator/v2"
	"github.com/knadh/stuffbin"
)

var (
	buildString   = "unknown"
	versionString = "unknown"
)

type Consts struct {
	RootURL           string   `json:"app.root_url"`
	ManifestURI       string   `json:"app.manifest_path"`
	WellKnownURI      string   `json:"app.wellknown_path"`
	DisallowedDomains []string `json:"crawl.disallowed_domains"`

	AdminUsername []byte `json:"app.admin_username"`
	AdminPassword []byte `json:"app.admin_password"`

	EnableCaptcha           bool   `json:"site.enable_captcha"`
	CaptchaComplexity       int64  `json:"site.captcha_complexity"`
	CaptchaKey              string `json:"-"`
	DefaultSubmissionstatus string `json:"site.default_submission_status"`

	HomeNumTags     int `json:"site.home_num_tags"`
	HomeNumProjects int `json:"site.home_num_projects"`

	DumpFileName string `json:"site.dump_filename"`
}

// App contains the "global" components that are passed around, especially through HTTP handlers.
type App struct {
	consts  Consts
	siteTpl *template.Template
	core    *core.Core
	search  *search.Search
	crawl   *crawl.Crawl
	schema  crawl.Schema
	pg      *paginator.Paginator

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
		install(migrationsList[len(migrationsList)-1].version,
			!ko.Bool("yes"),
			ko.Bool("install-db"),
			ko.Bool("install-search"),
			app,
			ko)
		return
	}
	if ko.Bool("upgrade") {
		upgrade(db, app.fs, !ko.Bool("yes"))
		os.Exit(0)
	}

	// Before the queries are prepared, see if there are pending upgrades.
	checkUpgrade(db)

	// Initialize queries and data handler.
	app.core = initCore(app.fs, db)
	app.schema = initSchema(ko)
	app.search = initSearch(ko)
	app.crawl = initCrawl(app.schema, app.core, app.search, ko)
	app.pg = initPaginator(ko)

	// Run the crawl mode.
	switch ko.String("mode") {
	case "crawl":
		app.crawl.Crawl()
		return
	case "sync-search":
		syncSearch(app.core, app.search, lo)
		return
	case "dump":
		dumpManifests(app.core, lo)
		return
	}

	// Initialize the echo HTTP server.
	srv := initHTTPServer(app, ko)

	lo.Printf("starting server on %s", ko.MustString("app.address"))
	if err := srv.Start(ko.MustString("app.address")); err != nil {
		lo.Fatalf("error starting HTTP server: %v", err)
	}
}
