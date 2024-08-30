package crawl

import (
	"errors"
	"log"
	"net/url"
	"sync"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
)

type Schema interface {
	Validate(v1.Manifest) (v1.Manifest, error)
	ParseManifest(b []byte, manifestURL string, checkProvenance bool) (v1.Manifest, error)
}

type DB interface {
	GetManifestsURLsByAge(age string, offsetID, limit int) ([]v1.ManifestURL, error)
	UpsertManifest(m v1.Manifest) (v1.Manifest, error)
	UpdateManifestStatus(id int, status string) error
}

type Opt struct {
	Workers         int    `json:"workers"`
	ManifestAge     string `json:"manifest_age"`
	BatchSize       int    `json:"batch_size"`
	CheckProvenance bool   `json:"check_provenance"`

	HTTP common.HTTPOpt
}

type Crawl struct {
	opt *Opt
	sc  Schema
	db  DB

	wg   *sync.WaitGroup
	jobs chan v1.ManifestURL

	hc  *common.HTTPClient
	log *log.Logger
}

var (
	ErrRatelimited = errors.New("host rate limited the request")
)

func New(o *Opt, sc Schema, db DB, l *log.Logger) *Crawl {
	return &Crawl{
		opt: o,
		sc:  sc,
		db:  db,
		hc:  common.NewHTTPClient(o.HTTP, l),

		wg:   &sync.WaitGroup{},
		jobs: make(chan v1.ManifestURL, o.BatchSize),
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

// FetchManifest fetches a given funding.json manifest, parses it, and returns.
func (c *Crawl) FetchManifest(manifest *url.URL) (v1.Manifest, error) {
	b, err := c.hc.Get(manifest)
	if err != nil {
		return v1.Manifest{}, err
	}

	return c.sc.ParseManifest(b, manifest.String(), c.opt.CheckProvenance)
}
