package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V1_1_0 performs the DB migrations for v.1.1.0.
func V1_1_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`
	DROP TABLE IF EXISTS reports CASCADE;
	CREATE TABLE IF NOT EXISTS reports (
		id                  SERIAL PRIMARY KEY,
		manifest_id         INTEGER REFERENCES manifests(id) ON DELETE CASCADE ON UPDATE CASCADE,
		reason              TEXT NOT NULL,
		created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	`)
	return err
}
