package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"path"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/Masterminds/sprig"
	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/crawl"
	"github.com/floss-fund/portal/internal/models"
	"github.com/floss-fund/portal/internal/search"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/goyesql/v2"
	goyesqlx "github.com/knadh/goyesql/v2/sqlx"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/paginator/v2"
	"github.com/knadh/stuffbin"
	"github.com/labstack/echo/v4"
	flag "github.com/spf13/pflag"
)

type Schema struct {
	schema *v1.Schema
}

func initConfig() {
	// Commandline flags.
	f := flag.NewFlagSet("config", flag.ContinueOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		fmt.Printf("floss.fund portal (%s) tool", versionString)
		os.Exit(0)
	}

	f.String("mode", "site", "site = runs the public portal | crawl = runs the background crawler | dump = dump raw manifest data to stdout")
	f.Bool("new-config", false, "generate a new sample config.toml file.")
	f.StringSlice("config", []string{"config.toml"},
		"path to one or more config files (will be merged in order)")
	f.Bool("install", false, "run first time DB installation")
	f.Bool("install-db", true, "run installation on PostgresDB")
	f.Bool("install-search", true, "run installation on TypeSense search")
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
		RootURL:                 ko.MustString("app.root_url"),
		AdminUsername:           ko.MustBytes("app.admin_username"),
		AdminPassword:           ko.MustBytes("app.admin_password"),
		ManifestURI:             ko.MustString("crawl.manifest_uri"),
		WellKnownURI:            ko.MustString("crawl.wellknown_uri"),
		DisallowedDomains:       ko.Strings("crawl.disallowed_domains"),
		EnableCaptcha:           ko.Bool("site.enable_captcha"),
		HomeNumTags:             ko.MustInt("site.home_num_tags"),
		HomeNumProjects:         ko.MustInt("site.home_num_projects"),
		DefaultSubmissionstatus: ko.MustString("site.default_submission_status"),
		DumpFileName:            ko.MustString("site.dump_filename"),
	}

	if c.EnableCaptcha {
		c.CaptchaComplexity = ko.MustInt64("site.captcha_complexity")

		b := make([]byte, 24) // 24 bytes will give 32 characters when base64 encoded
		_, err := rand.Read(b)
		if err != nil {
			lo.Fatalf("error generating captcha key: %v", err)
		}
		c.CaptchaKey = base64.URLEncoding.EncodeToString(b)[:32]
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

	// Generate a random string for cache busting in templates.
	b := md5.Sum([]byte(time.Now().String()))
	srv.Renderer = &tplRenderer{
		tpl:      initSiteTemplates(ko.MustString("app.template_dir")),
		RootURL:  ko.MustString("app.root_url"),
		AssetVer: fmt.Sprintf("%x", b)[0:10],
	}

	// Register app (*App) to be injected into all HTTP handlers.
	srv.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("app", app)
			return next(c)
		}
	})

	initHandlers(ko, srv)

	return srv
}

func initCore(fs stuffbin.FileSystem, db *sqlx.DB) *core.Core {
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

	return core.New(&q, db.Unsafe(), opt, lo)
}

func initCrawl(sc crawl.Schema, co *core.Core, s *search.Search, ko *koanf.Koanf) *crawl.Crawl {
	opt := crawl.Opt{
		Workers:         ko.MustInt("crawl.workers"),
		ManifestAge:     ko.MustString("crawl.manifest_age"),
		BatchSize:       ko.MustInt("crawl.batch_size"),
		CheckProvenance: ko.Bool("crawl.check_provenance"),
		MaxCrawlErrors:  ko.MustInt("crawl.max_crawl_errors"),

		HTTP: initHTTPOpt(),
	}

	// When the crawler updates manifests, fire the callback to search results.
	cb := &crawl.Callbacks{
		OnManifestUpdate: func(m models.ManifestData, status string) {
			updateSearchRecord(m, status, s)
		},
	}

	return crawl.New(&opt, sc, cb, co, lo)
}

func initPaginator(ko *koanf.Koanf) *paginator.Paginator {
	perPage := ko.MustInt("search.per_page")
	pgOpt := paginator.Default()
	pgOpt.DefaultPerPage = perPage
	pgOpt.MaxPerPage = perPage
	return paginator.New(pgOpt)
}

func initSchema(ko *koanf.Koanf) crawl.Schema {
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

	// Since the portal has it's own models.Manifest (with additional fields),
	// have to use a simple abstraction to pass the underlying v1 schema to the
	// schema validator.
	return &Schema{schema: sc}
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

func initSearch(ko *koanf.Koanf) *search.Search {
	opt := search.Opt{
		RootURL: ko.MustString("search.root_url"),
		APIKey:  ko.MustString("search.api_key"),
		PerPage: ko.MustInt("search.per_page"),

		HTTP: initHTTPOpt(),
	}

	return search.New(opt, lo)
}

func initSiteTemplates(dirPath string) *template.Template {
	// Create a new template set
	tmpl := template.New("")

	// Add Sprig functions to the template set
	tmpl.Funcs(sprig.FuncMap())
	tmpl.Funcs(template.FuncMap{
		"Nl2br": func(input string) template.HTML {
			input = reMultiLines.ReplaceAllString(html.EscapeString(input), "\n\n")
			input = strings.Replace(input, "\n", "<br />", -1)
			return template.HTML(input)
		},

		"HasField": func(v interface{}, field string) bool {
			val := reflect.ValueOf(v)

			// If it's a reference, get the underlying.
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}

			// Check if it's a struct.
			if val.Kind() != reflect.Struct {
				return false
			}

			_, exists := val.Type().FieldByName(field)
			return exists
		},
	})

	// Parse all HTML files that match the pattern
	tpl, err := tmpl.ParseGlob(path.Join(dirPath, "*.html"))
	if err != nil {
		log.Fatalf("error parsing templates in %s: %v", dirPath, err)
	}
	tpl, err = tpl.ParseGlob(path.Join(dirPath, "partials/*.html"))
	if err != nil {
		log.Fatalf("error parsing templates in %s: %v", dirPath, err)
	}

	return tpl
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

func (s *Schema) Validate(m models.ManifestData) (models.ManifestData, error) {
	schemaManifest, err := s.schema.Validate(m.Manifest)
	if err != nil {
		return m, err
	}
	m.Manifest = schemaManifest
	return m, nil
}

func (s *Schema) ParseManifest(b []byte, manifestURL string, checkProvenance bool) (models.ManifestData, error) {
	schemaManifest, err := s.schema.ParseManifest(b, manifestURL, checkProvenance)
	if err != nil {
		return models.ManifestData{}, err
	}
	return models.ManifestData{Manifest: schemaManifest}, nil
}
