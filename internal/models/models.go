package models

import (
	"net/url"
	"time"

	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
)

type ManifestJob struct {
	ID           int       `json:"id" db:"id"`
	UUID         string    `json:"uuid" db:"uuid"`
	URL          string    `json:"url" db:"url"`
	LastModified time.Time `json:"updated_at" db:"updated_at"`

	URLobj *url.URL `json:"-" db:"-"`
}

//easyjson:json
type Manifest struct {
	// This is added internally and is not expected in the manifest itself.
	ID   int    `json:"-" db:"id"`
	UUID string `json:"-" db:"uuid"`

	v1.Manifest
}
