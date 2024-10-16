package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/models"
	"github.com/jmoiron/sqlx"
)

const maxURISize = 40
const maxURLLen = 200

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
	GetPendingManifests  *sqlx.Stmt `query:"get-pending-manifests"`
}

type Core struct {
	q   *Queries
	opt Opt
	hc  *http.Client
	log *log.Logger
}

var (
	ErrNotFound = errors.New("not found")
)

func New(q *Queries, o Opt, lo *log.Logger) *Core {
	return &Core{
		q:   q,
		log: lo,
	}
}

// fillRawManifest fills in the raw JSON fields in the manifest data.
func (d *Core) fillRawManifest(out *models.ManifestData) error {
	// Entity.
	if err := out.Entity.UnmarshalJSON(out.EntityRaw); err != nil {
		return fmt.Errorf("error unmarshalling entity: %w", err)
	}

	// Funding.
	if err := out.Funding.UnmarshalJSON(out.FundingRaw); err != nil {
		return fmt.Errorf("error unmarshalling funding: %w", err)
	}

	// Create a funding map channel for easy lookups.
	out.Channels = make(map[string]v1.Channel)
	for _, c := range out.Funding.Channels {
		out.Channels[c.GUID] = c
	}

	// Unmarshal the entity URL. DB names and local names don't match,
	// and it's a nested structure. Sucks.
	var entity models.EntityURL
	if err := entity.UnmarshalJSON(out.EntityRaw); err != nil {
		return fmt.Errorf("error unmarshalling entity URL: %w", err)
	}

	if u, err := common.IsURL("url", entity.WebpageURL, maxURLLen); err != nil {
		return fmt.Errorf("error parsing entity URL %s: %w", entity.WebpageURL, err)
	} else {
		out.Entity.WebpageURL = v1.URL{URL: entity.WebpageURL, URLobj: u}
	}

	// Fill in the projects.
	if err := out.Projects.UnmarshalJSON(out.ProjectsRaw); err != nil {
		return fmt.Errorf("error unmarshalling projects: %w", err)
	}

	// Unmarshal project URLs. DB names and local names don't match,
	// and it's a nested structure. This sucks.
	var prjURLs models.ProjectURLs
	if err := prjURLs.UnmarshalJSON(out.ProjectsRaw); err != nil {
		return fmt.Errorf("error unmarshalling project URLs: %w", err)
	}

	for n, p := range prjURLs {
		if u, err := common.IsURL("url", p.WebpageURL, maxURLLen); err != nil {
			return fmt.Errorf("error parsing entity URL: %w", err)
		} else {
			out.Projects[n].WebpageURL = v1.URL{URL: p.WebpageURL, URLobj: u}
		}

		if u, err := common.IsURL("url", p.RepositoryURL, maxURLLen); err != nil {
			return fmt.Errorf("error parsing entity URL (%s): %w", p.RepositoryURL, err)
		} else {
			out.Projects[n].RepositoryURL = v1.URL{URL: p.RepositoryURL, URLobj: u}
		}
	}

	return nil
}

// GetManifest retrieves a manifest.
func (d *Core) GetManifest(id int, guid string) (models.ManifestData, error) {
	var (
		out models.ManifestData
	)

	// Get the manifest. entity{} and projects[{}] are retrieved
	// as JSON fields that need to be manually unmarshalled.
	if err := d.q.GetManifest.Get(&out, id, guid); err != nil {
		if err == sql.ErrNoRows {
			return out, ErrNotFound
		}

		d.log.Printf("error fetching manifest: %d: %v", id, err)
		return out, err
	}

	if err := d.fillRawManifest(&out); err != nil {
		d.log.Printf("error filling manifest: %d: %v", id, err)
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
		url, err := common.IsURL("url", u.URL, maxURLLen)
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

// GetPendingManifests returns a list of pending manifests.
func (d *Core) GetPendingManifests(limit, offset int) ([]models.ManifestData, error) {
	var out []models.ManifestData
	if err := d.q.GetPendingManifests.Select(&out, limit, offset); err != nil {
		d.log.Printf("error fetching pending manifests: %v", err)
		return nil, err
	}

	for i := range out {
		if err := d.fillRawManifest(&out[i]); err != nil {
			d.log.Printf("error filling manifest: %d: %v", out[i].ID, err)
			continue
		}
	}

	return out, nil
}

// MakeGUID takes a URL and creates a string "guid" in the form of
// @$host/$uri (last 3 parts, if there are, capped at 40 chars).
func MakeGUID(u *url.URL) string {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	// Get the last 3 parts (or fewer if there aren't 3).
	last := parts
	if len(parts) > 3 {
		last = parts[len(parts)-3:]
	}

	// Join the last parts.
	uri := strings.Join(last, "/")

	// If the URI is long, cut it to size from the start.
	if len(uri) > 40 {
		uri = "--" + uri[len(uri)-40:]
	}

	guid := "@" + u.Host + "/" + uri
	return guid
}
