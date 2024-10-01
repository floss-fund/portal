package core

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/floss-fund/go-funding-json/common"
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
	ManifestStatusBlocked  = "blocked"
)

// Queries contains prepared DB queries.
type Queries struct {
	UpsertManifest       *sqlx.Stmt `query:"upsert-manifest"`
	GetManifestStatus    *sqlx.Stmt `query:"get-manifest-status"`
	GetForCrawling       *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus *sqlx.Stmt `query:"update-manifest-status"`
	GetTopTags           *sqlx.Stmt `query:"get-top-tags"`
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

// GetManifestStatus checks whether a given manifest URL exists in the databse.
// If one exists, its status is returned.
func (d *Core) GetManifestStatus(url string) (string, error) {
	var status string
	if err := d.q.GetManifestStatus.Get(&status, url); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}

		d.log.Printf("error checking manifest status: %v", err)
		return "", err
	}

	return status, nil
}

// UpsertManifest upserts an entry into the database.
func (d *Core) UpsertManifest(m v1.Manifest) error {
	body, err := m.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest: %v", err)
		return err
	}

	if _, err := d.q.UpsertManifest.Exec(json.RawMessage(body), m.URL.URL, json.RawMessage("{}"), ManifestStatusPending, ""); err != nil {
		d.log.Printf("error upsering manifest: %v", err)
		return err
	}

	return nil
}

// GetManifestForCrawling retrieves manifest URLs that need to be crawled again. It returns records in batches of limit length,
// continued from the last processed row ID which is the offsetID.
func (d *Core) GetManifestForCrawling(age string, offsetID, limit int) ([]models.ManifestJob, error) {
	var out []models.ManifestJob
	if err := d.q.GetForCrawling.Select(&out, offsetID, age, limit); err != nil {
		d.log.Printf("error fetching URLs for crawling: %v", err)
		return nil, err
	}

	for n, u := range out {
		url, err := common.IsURL("url", u.URL, 1024)
		if err != nil {
			d.log.Printf("error parsing url %v: ", err)
			continue
		}

		u.URLobj = url
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

// GetTopTags returns top N tags referenced across projects.
func (d *Core) GetTopTags(limit int) ([]string, error) {
	res := []struct {
		Tag string `db:"tag"`
	}{}
	if err := d.q.GetTopTags.Select(&res, limit); err != nil {
		d.log.Printf("error fetching top tags: %v", err)
		return nil, err
	}

	tags := make([]string, 0, len(res))
	for _, t := range res {
		tags = append(tags, t.Tag)
	}

	return tags, nil
}
