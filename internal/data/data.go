package data

import "github.com/jmoiron/sqlx"

// Queries contains prepared DB queries.
type Queries struct {
	Search *sqlx.Stmt `query:"search"`
}

// Data represents the dictionary search interface.
type Data struct {
	queries *Queries
}

// New returns an instance of the search interface.
func New(q *Queries) *Data {
	return &Data{
		queries: q,
	}
}
