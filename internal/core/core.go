package core

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/floss-fund/go-funding-json/common"
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
	GetManifest          *sqlx.Stmt `query:"get-manifest"`
	GetManifestStatus    *sqlx.Stmt `query:"get-manifest-status"`
	GetForCrawling       *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus *sqlx.Stmt `query:"update-manifest-status"`
	UpdateCrawlError     *sqlx.Stmt `query:"update-crawl-error"`
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

// GetManifest retrieves a manifest.
func (d *Core) GetManifest(id int) (models.ManifestData, error) {
	var (
		out models.ManifestData
	)

	// Get the manifest. entity{} and projects[{}] are retrieved
	// as JSON fields that need to be manually unmarshalled.
	if err := d.q.GetManifest.Get(&out, id); err != nil {
		if err == sql.ErrNoRows {
			return out, nil
		}

		d.log.Printf("error fetching manifest: %d: %v", id, err)
		return out, err
	}

	if err := out.Entity.UnmarshalJSON(out.EntityRaw); err != nil {
		d.log.Printf("error unmarshalling entity: %d: %v", id, err)
		return out, err
	}

	if err := out.Projects.UnmarshalJSON(out.ProjectsRaw); err != nil {
		d.log.Printf("error unmarshalling projects: %d: %v", id, err)
		return out, err
	}

	return out, nil
}

// GetManifestStatus checks whether a given manifest URL exists in the databse.
// If one exists, its status is returned.
func (d *Core) GetManifestStatus(url string) (string, error) {
	var status string
	if err := d.q.GetManifestStatus.Get(&status, url); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}

		d.log.Printf("error checking manifest status: %s: %v", url, err)
		return "", err
	}

	return status, nil
}

// UpsertManifest upserts an entry into the database.
func (d *Core) UpsertManifest(m models.ManifestData) error {
	body, err := m.Manifest.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest: %s: %v", m.URL, err)
		return err
	}

	if _, err := d.q.UpsertManifest.Exec(json.RawMessage(body), m.Manifest.URL.URL, m.GUID, json.RawMessage("{}"), ManifestStatusPending, ""); err != nil {
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
			d.log.Printf("error parsing url: %s: %v: ", u.URL, err)
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
		d.log.Printf("error updating manifest status: %d: %v", id, err)
		return err
	}

	return nil
}

// UpdateManifestCrawlError updates a manifest's crawl error count and sets
// it to 'disabled' if it exceeds the given limit.
func (d *Core) UpdateManifestCrawlError(id int, message string, maxErrors int) (string, error) {
	var status string
	if err := d.q.UpdateCrawlError.Get(&status, id, message, maxErrors); err != nil {
		d.log.Printf("error updating manifest crawl error status: %d: %v", id, err)
		return "", err
	}

	return status, nil
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
