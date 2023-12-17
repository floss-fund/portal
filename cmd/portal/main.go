package main

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"floss.fund/portal/internal/data"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/goyesql/v2"
	goyesqlx "github.com/knadh/goyesql/v2/sqlx"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
	flag "github.com/spf13/pflag"
)

var (
	buildString   = "unknown"
	versionString = "unknown"
)

type Consts struct {
	RootURL string
}

// App contains the "global" components that are passed around, especially through HTTP handlers.
type App struct {
	consts  Consts
	siteTpl *template.Template
	data    *data.Data
	queries *data.Queries

	db *sqlx.DB
	fs stuffbin.FileSystem
	lo *log.Logger
}

var (
	lo = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	ko = koanf.New(".")
)

func initConfig() {
	// Commandline flags.
	f := flag.NewFlagSet("config", flag.ContinueOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		fmt.Printf("floss.fund portal (%s) tool", versionString)
		os.Exit(0)
	}

	f.Bool("new-config", false, "generate a new sample config.toml file.")
	f.StringSlice("config", []string{"config.toml"},
		"path to one or more config files (will be merged in order)")
	f.Bool("install", false, "run first time DB installation")
	f.Bool("upgrade", false, "upgrade database to the current version")
	f.Bool("yes", false, "assume 'yes' to prompts during --install/upgrade")
	f.Bool("version", false, "current version of the build")

	if err := f.Parse(os.Args[1:]); err != nil {
		lo.Fatalf("error parsing flags: %v", err)
	}

	if ok, _ := f.GetBool("version"); ok {
		fmt.Println(buildString)
		os.Exit(0)
	}

	// Generate new config file.
	if ok, _ := f.GetBool("new-config"); ok {
		if err := generateNewFiles(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("config.toml generated. Edit and run --install.")
		os.Exit(0)
	}

	// Load config files.
	cFiles, _ := f.GetStringSlice("config")
	for _, f := range cFiles {
		lo.Printf("reading config: %s", f)

		if err := ko.Load(file.Provider(f), toml.Parser()); err != nil {
			fmt.Printf("error reading config: %v", err)
			os.Exit(1)
		}
	}

	if err := ko.Load(posflag.Provider(f, ".", ko), nil); err != nil {
		lo.Fatalf("error loading config: %v", err)
	}
}

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

	// Install schema.
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

	// Load SQL queries.
	qB, err := app.fs.Read("/queries.sql")
	if err != nil {
		lo.Fatalf("error reading queries.sql: %v", err)
	}

	qMap, err := goyesql.ParseBytes(qB)
	if err != nil {
		lo.Fatalf("error loading SQL queries: %v", err)
	}

	// Map queries to the query container.
	var q data.Queries
	if err := goyesqlx.ScanToStruct(&q, qMap, db.Unsafe()); err != nil {
		lo.Fatalf("no SQL queries loaded: %v", err)
	}

	app.data = data.New(&q)
	app.queries = &q

	// Initialize the echo HTTP server.
	srv := initHTTPServer(app, ko)

	lo.Printf("starting server on %s", ko.MustString("app.address"))
	if err := srv.Start(ko.MustString("app.address")); err != nil {
		lo.Fatalf("error starting HTTP server: %v", err)
	}
}
