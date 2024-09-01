package core

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/models"
	"github.com/jmoiron/sqlx"
)

type Opt struct {
}

const (
	ManifestStatusPending  = "pending"
	ManifestStatusActive   = "active"
	ManifestStatusExpiring = "expiring"
	ManifestStatusDisabled = "disabled"
)

// Queries contains prepared DB queries.
type Queries struct {
	UpsertManifest       *sqlx.Stmt `query:"upsert-manifest"`
	GetForCrawling       *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus *sqlx.Stmt `query:"update-manifest-status"`
}

type Core struct {
	q   *Queries
	opt Opt
	hc  *http.Client
	log *log.Logger
}

func New(q *Queries, o Opt, lo *log.Logger) *Core {
	return &Core{
		q:   q,
		log: lo,
	}
}

// UpsertManifest upserts an entry into the database.
func (d *Core) UpsertManifest(m v1.Manifest) (v1.Manifest, error) {
	body, err := m.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest: %v", err)
		return m, err
	}

	entity, err := m.Entity.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest.entity: %v", err)
		return m, err
	}

	projects, err := m.Projects.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest.projects: %v", err)
		return m, err
	}

	channels, err := m.Funding.Channels.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest.funding.channels: %v", err)
		return m, err
	}

	plans, err := m.Funding.Plans.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest.funding.plans: %v", err)
		return m, err
	}

	history, err := m.Funding.History.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest.funding.plans: %v", err)
		return m, err
	}

	if _, err := d.q.UpsertManifest.Exec(m.Version, m.URL, body, entity, projects, channels, plans, history, json.RawMessage("{}"), ManifestStatusPending); err != nil {
		d.log.Printf("error upsering manifest: %v", err)
		return m, err
	}

	return m, nil
}

// GetManifestURLsByAge retrieves manifest URLs that need to be crawled again. It returns records in batches of limit length,
// continued from the last processed row ID which is the offsetID.
func (d *Core) GetManifestURLsByAge(age string, offsetID, limit int) ([]models.ManifestURL, error) {
	var out []models.ManifestURL
	if err := d.q.GetForCrawling.Select(&out, offsetID, age, limit); err != nil {
		d.log.Printf("error fetching URLs for crawling: %v", err)
		return nil, err
	}

	for n, u := range out {
		p, err := url.Parse(u.URL)
		if err != nil {
			d.log.Printf("error parsing url %v: ", err)
			continue
		}

		u.URLobj = p
		out[n] = u
	}

	return out, nil
}

// UpdateManifestStatus updates a manifest's status.
func (d *Core) UpdateManifestStatus(id int, status string) error {
	if _, err := d.q.UpdateManifestStatus.Exec(id, status); err != nil {
		d.log.Printf("error updating manifest status: %v", err)
		return err
	}

	return nil
}
