package core

import (
	"encoding/json"
	"log"
	"net/http"

	v1 "floss.fund/portal/internal/schemas/v1"
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
	UpsertManifest *sqlx.Stmt `query:"upsert-manifest"`
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

	if _, err := d.q.UpsertManifest.Exec(m.Version, m.URL, m.Body, entity, projects, channels, plans, history, json.RawMessage("{}"), ManifestStatusPending); err != nil {
		d.log.Printf("error upsering manifest: %v", err)
		return m, err
	}

	return m, nil
}
