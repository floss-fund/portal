package crawl

import (
	"errors"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/floss-fund/go-funding-json/common"
	"github.com/floss-fund/portal/internal/models"
)

type Schema interface {
	Validate(models.ManifestData) (models.ManifestData, error)
	ParseManifest(b []byte, manifestURL string, checkProvenance bool) (models.ManifestData, error)
}

type DB interface {
	GetManifestForCrawling(age string, offsetID, limit int) ([]models.ManifestJob, error)
	UpsertManifest(m models.ManifestData) error
	UpdateManifestCrawlError(id int, message string, maxErrors int) (string, error)
}

type Opt struct {
	Workers         int    `json:"workers"`
	ManifestAge     string `json:"manifest_age"`
	BatchSize       int    `json:"batch_size"`
	CheckProvenance bool   `json:"check_provenance"`
	MaxCrawlErrors  int    `json:"max_crawl_errors"`

	HTTP common.HTTPOpt
}

type Crawl struct {
	opt       *Opt
	sc        Schema
	Callbacks *Callbacks
	db        DB

	wg   *sync.WaitGroup
	jobs chan models.ManifestJob

	hc  *common.HTTPClient
	log *log.Logger
}

type Callbacks struct {
	OnManifestUpdate func(m models.ManifestData, status string)
}

var (
	ErrRatelimited = errors.New("host rate limited the request")
)

func New(o *Opt, sc Schema, cb *Callbacks, db DB, l *log.Logger) *Crawl {
	return &Crawl{
		opt:       o,
		sc:        sc,
		Callbacks: cb,
		db:        db,
		hc:        common.NewHTTPClient(o.HTTP, l),

		wg:   &sync.WaitGroup{},
		jobs: make(chan models.ManifestJob, o.BatchSize),
		log:  l,
	}
}

func (c *Crawl) Crawl() error {
	for n := 0; n < c.opt.Workers; n++ {
		c.wg.Add(1)

		go c.worker()
	}

	go c.dbWorker()

	c.wg.Wait()
	return nil
}

// IsManifestModified sends a head request to a manifest URL and
// indicates whether it's been updated (true=needs re-crawling).
func (c *Crawl) IsManifestModified(manifest *url.URL, lastModified time.Time) (bool, error) {
	hdr, err := c.hc.Head(manifest)
	if err != nil {
		return false, err
	}

	last := hdr.Get("Last-Modified")

	// Header doesn't exist. Re-crawl.
	if last == "" {
		return true, nil
	}

	t, err := time.Parse(time.RFC1123, last)
	if err != nil {
		return false, err
	}

	return lastModified.Before(t), nil
}

// FetchManifest fetches a given funding.json manifest, parses it, and returns.
func (c *Crawl) FetchManifest(manifest *url.URL) (models.ManifestData, error) {
	b, err := c.hc.Get(common.TransformURLOrigin(manifest))
	if err != nil {
		return models.ManifestData{}, err
	}

	return c.sc.ParseManifest(b, manifest.String(), c.opt.CheckProvenance)
}
