package models

import (
	"net/url"
	"time"
)

type ManifestJob struct {
	ID           int       `json:"id" db:"id"`
	URL          string    `json:"url" db:"url"`
	LastModified time.Time `json:"updated_at" db:"updated_at"`

	URLobj *url.URL `json:"-" db:"-"`
}
