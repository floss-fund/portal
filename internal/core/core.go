package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "floss.fund/portal/internal/schemas/v1"
	"github.com/jmoiron/sqlx"
)

type Opt struct {
	CrawlUseragent    string
	CrawlMaxHostConns int
	CrawlReqTimeout   time.Duration
	CrawlRetries      int
}

// Queries contains prepared DB queries.
type Queries struct {
	UpsertEntry *sqlx.Stmt `query:"upsert-entry"`
}

type Core struct {
	queries *Queries
	opt     Opt
	hc      *http.Client
}

func New(q *Queries, o Opt) *Core {
	return &Core{
		queries: q,
		hc: &http.Client{
			Timeout: o.CrawlReqTimeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost:   o.CrawlMaxHostConns,
				MaxConnsPerHost:       o.CrawlMaxHostConns,
				ResponseHeaderTimeout: o.CrawlReqTimeout,
				IdleConnTimeout:       o.CrawlReqTimeout,
			},
		},
	}
}

// UpsertEntry upserts an entry into the database.
func (d *Core) UpsertEntry(e v1.Manifest) (v1.Manifest, error) {
	return e, nil
}

func (c *Core) exec(method, rURL string, reqBody []byte, headers http.Header) error {
	var (
		err      error
		postBody io.Reader
	)

	// Encode POST / PUT params.
	if method == http.MethodPost || method == http.MethodPut {
		postBody = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequest(method, rURL, postBody)
	if err != nil {
		return err
	}

	if headers != nil {
		req.Header = headers
	} else {
		req.Header = http.Header{}
	}
	req.Header.Set("User-Agent", "listmonk")

	// If a content-type isn't set, set the default one.
	if req.Header.Get("Content-Type") == "" {
		if method == http.MethodPost || method == http.MethodPut {
			req.Header.Add("Content-Type", "application/json")
		}
	}

	// If the request method is GET or DELETE, add the params as QueryString.
	if method == http.MethodGet || method == http.MethodDelete {
		req.URL.RawQuery = string(reqBody)
	}

	r, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		// Drain and close the body to let the Transport reuse the connection
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 response from URL: %d", r.StatusCode)
	}

	return nil
}
