package core

import (
	"net/http"

	v1 "floss.fund/portal/internal/schemas/v1"
	"github.com/jmoiron/sqlx"
)

type Opt struct {
}

// Queries contains prepared DB queries.
type Queries struct {
	UpsertManifest *sqlx.Stmt `query:"upsert-manifest"`
}

type Core struct {
	queries *Queries
	opt     Opt
	hc      *http.Client
}

func New(q *Queries, o Opt) *Core {
	return &Core{
		queries: q,
	}
}

// UpsertManifest upserts an entry into the database.
func (d *Core) UpsertManifest(e v1.Manifest) (v1.Manifest, error) {
	return e, nil
}
