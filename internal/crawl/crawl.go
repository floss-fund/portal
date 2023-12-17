package crawl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "floss.fund/portal/internal/schemas/v1"
)

type Schema interface {
	Validate(v1.Manifest) error
}

type Opt struct {
	Useragent    string
	MaxHostConns int
	ReqTimeout   time.Duration
	Retries      int
}

type Crawl struct {
	opt Opt
	sc  Schema
	hc  *http.Client
}

func New(o Opt, sc Schema) *Crawl {
	return &Crawl{
		sc: sc,
		hc: &http.Client{
			Timeout: o.ReqTimeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost:   o.MaxHostConns,
				MaxConnsPerHost:       o.MaxHostConns,
				ResponseHeaderTimeout: o.ReqTimeout,
				IdleConnTimeout:       o.ReqTimeout,
			},
		},
	}
}

// FetchManifest fetches a given funding.json manifest, parses it, and returns.
func (c *Crawl) FetchManifest(u string) (v1.Manifest, error) {
	var (
		body []byte
		err  error
	)

	h := http.Header{}
	h.Set("Useragent", c.opt.Useragent)

	for n := 0; n < c.opt.Retries; n++ {
		body, err = c.exec(http.MethodGet, u, nil, h)
		if err == nil {
			break
		}
	}

	if err != nil {
		return v1.Manifest{}, err
	}

	var out v1.Manifest
	if err := out.UnmarshalJSON(body); err != nil {
		return out, fmt.Errorf("error parsing JSON body: %v", err)
	}

	return out, nil
}

// ParseManifest parses a given JSON body, validates it, and returns the manifest.
func (c *Crawl) ParseManifest(b []byte) (v1.Manifest, error) {
	var out v1.Manifest
	if err := out.UnmarshalJSON(b); err != nil {
		return out, fmt.Errorf("error parsing JSON body: %v", err)
	}

	if err := c.sc.Validate(out); err != nil {
		return out, err
	}

	return out, nil
}

func (c *Crawl) exec(method, rURL string, reqBody []byte, headers http.Header) ([]byte, error) {
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
		return nil, err
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
		return nil, err
	}
	defer func() {
		// Drain and close the body to let the Transport reuse the connection
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from URL: %d", r.StatusCode)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
