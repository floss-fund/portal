package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"unicode"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/crawl"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/goyesql/v2"
	goyesqlx "github.com/knadh/goyesql/v2/sqlx"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
	"github.com/labstack/echo/v4"
	flag "github.com/spf13/pflag"
)

func initConfig() {
	// Commandline flags.
	f := flag.NewFlagSet("config", flag.ContinueOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		fmt.Printf("floss.fund portal (%s) tool", versionString)
		os.Exit(0)
	}

	f.String("mode", "site", "site = runs the public portal | crawl = runs the background crawler")
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

func initConstants(ko *koanf.Koanf) Consts {
	c := Consts{
		RootURL:      ko.MustString("app.root_url"),
		ManifestURI:  ko.MustString("crawl.manifest_uri"),
		WellKnownURI: ko.MustString("crawl.wellknown_uri"),
	}

	return c
}

// initDB initializes a database connection.
func initDB(host string, port int, user, pwd, dbName string) *sqlx.DB {
	db, err := sqlx.Connect("postgres",
		fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, pwd, dbName))
	if err != nil {
		lo.Fatalf("error initializing DB: %v", err)
	}

	return db
}

// initFS initializes the stuffbin FileSystem to provide
// access to bunded static assets to the app.
func initFS() stuffbin.FileSystem {
	path, err := os.Executable()
	if err != nil {
		lo.Fatalf("error getting executable path: %v", err)
	}

	fs, err := stuffbin.UnStuff(path)
	if err == nil {
		return fs
	}

	// Running in local mode. Load the required static assets into
	// the in-memory stuffbin.FileSystem.
	lo.Printf("unable to initialize embedded filesystem: %v", err)
	lo.Printf("using local filesystem for static assets")

	files := []string{
		"config.sample.toml",
		"queries.sql",
		"schema.sql",
	}

	fs, err = stuffbin.NewLocalFS("/", files...)
	if err != nil {
		lo.Fatalf("failed to load local static files: %v", err)
	}

	return fs
}

func initHTTPServer(app *App, ko *koanf.Koanf) *echo.Echo {
	srv := echo.New()
	srv.Debug = true
	srv.HideBanner = true

	// Register app (*App) to be injected into all HTTP handlers.
	srv.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("app", app)
			return next(c)
		}
	})

	initHandlers(srv)

	return srv
}

func initCore(fs stuffbin.FileSystem, db *sqlx.DB, ko *koanf.Koanf) *core.Core {
	// Load SQL queries.
	qB, err := fs.Read("/queries.sql")
	if err != nil {
		lo.Fatalf("error reading queries.sql: %v", err)
	}

	qMap, err := goyesql.ParseBytes(qB)
	if err != nil {
		lo.Fatalf("error loading SQL queries: %v", err)
	}

	// Map queries to the query container.
	var q core.Queries
	if err := goyesqlx.ScanToStruct(&q, qMap, db.Unsafe()); err != nil {
		lo.Fatalf("no SQL queries loaded: %v", err)
	}

	opt := core.Opt{}

	return core.New(&q, opt, lo)
}

func initCrawl(sc crawl.Schema, co *core.Core, ko *koanf.Koanf) *crawl.Crawl {
	opt := crawl.Opt{
		Workers:         ko.MustInt("crawl.workers"),
		ManifestAge:     ko.MustString("crawl.manifest_age"),
		BatchSize:       ko.MustInt("crawl.batch_size"),
		CheckProvenance: ko.Bool("crawl.check_provenance"),

		HTTP: initHTTPOpt(),
	}

	return crawl.New(&opt, sc, co, lo)
}

func initSchema(ko *koanf.Koanf) *v1.Schema {
	// SPDX license index.
	licenses := make(map[string]string)
	if b, err := os.ReadFile(ko.MustString("data_files.spdx")); err != nil {
		log.Fatalf("error reading spdx file: %v", err)
	} else {
		o := struct {
			Licenses []struct {
				Name string `json:"name"`
				ID   string `json:"licenseId"`
			} `json:"licenses"`
		}{}

		if err := json.Unmarshal(b, &o); err != nil {
			lo.Fatalf("error unmarshalling spdx file: %v", err)
		}

		for _, l := range o.Licenses {
			licenses[l.ID] = l.Name
		}
	}

	// Programming language list.
	langs := make(map[string]string)
	if b, err := os.ReadFile(ko.MustString("data_files.languages")); err != nil {
		log.Fatalf("error reading programming languages file: %v", err)
	} else {
		if err := json.Unmarshal(b, &langs); err != nil {
			lo.Fatalf("error unmarshalling programming languages file: %v", err)
		}
	}

	// Currencies list.
	currencies := make(map[string]string)
	if b, err := os.ReadFile(ko.MustString("data_files.currencies")); err != nil {
		log.Fatalf("error reading currencies file: %v", err)
	} else {
		if err := json.Unmarshal(b, &currencies); err != nil {
			lo.Fatalf("error unmarshalling currencies file: %v", err)
		}
	}

	// Initialize schema.
	sc := v1.New(&v1.Opt{
		WellKnownURI:         ko.MustString("crawl.wellknown_uri"),
		Licenses:             licenses,
		ProgrammingLanguages: langs,
		Currencies:           currencies,
	}, initHTTPOpt(), lo)

	return sc
}

func initHTTPOpt() common.HTTPOpt {
	return common.HTTPOpt{
		MaxHostConns: ko.MustInt("crawl.max_host_conns"),
		Retries:      ko.MustInt("crawl.retries"),
		RetryWait:    ko.MustDuration("crawl.retry_wait"),
		ReqTimeout:   ko.MustDuration("crawl.req_timeout"),
		MaxBytes:     ko.MustInt64("crawl.max_bytes"),
		UserAgent:    ko.MustString("crawl.useragent"),
	}
}

func generateNewFiles() error {
	if _, err := os.Stat("config.toml"); !os.IsNotExist(err) {
		return errors.New("config.toml exists. Remove it to generate a new one")
	}

	// Initialize the static file system into which all
	// required static assets (.sql, .js files etc.) are loaded.
	fs := initFS()

	// Generate config file.
	b, err := fs.Read("config.sample.toml")
	if err != nil {
		return fmt.Errorf("error reading sample config (is binary stuffed?): %v", err)
	}

	// Inject a random password.
	p := make([]byte, 12)
	rand.Read(p)
	pwd := []byte(fmt.Sprintf("%x", p))

	for i, c := range pwd {
		if mrand.Intn(4) == 1 {
			pwd[i] = byte(unicode.ToUpper(rune(c)))
		}
	}

	b = bytes.Replace(b, []byte("dictpress_admin_password"), pwd, -1)

	if err := ioutil.WriteFile("config.toml", b, 0644); err != nil {
		return err
	}

	return nil
}
