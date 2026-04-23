package pubmed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const baseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

type Client struct {
	http    *http.Client
	tool    string
	email   string
	apiKey  string
	limiter *limiter
}

func NewClient(tool, email, apiKey string) *Client {
	interval := 334 * time.Millisecond
	if apiKey != "" {
		interval = 100 * time.Millisecond
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		tool:    tool,
		email:   email,
		apiKey:  apiKey,
		limiter: newLimiter(interval),
	}
}

type SearchResult struct {
	Count    int
	PMIDs    []string
	RetMax   int
	RetStart int
}

type esearchResponse struct {
	ESearchResult struct {
		Count    string   `json:"count"`
		RetMax   string   `json:"retmax"`
		RetStart string   `json:"retstart"`
		IDList   []string `json:"idlist"`
	} `json:"esearchresult"`
}

// Search calls esearch.fcgi. mindate/maxdate use YYYY/MM/DD format and filter on EDAT.
// Pass empty strings to skip date filtering.
func (c *Client) Search(ctx context.Context, term, mindate, maxdate string, retmax, retstart int) (*SearchResult, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("term", term)
	q.Set("retmode", "json")
	q.Set("retmax", strconv.Itoa(retmax))
	q.Set("retstart", strconv.Itoa(retstart))
	if mindate != "" {
		q.Set("datetype", "edat")
		q.Set("mindate", mindate)
		q.Set("maxdate", maxdate)
	}
	q.Set("tool", c.tool)
	q.Set("email", c.email)
	if c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}

	u := baseURL + "/esearch.fcgi?" + q.Encode()

	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, lastErr = c.http.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
			resp = nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("esearch: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("esearch: status %d: %s", resp.StatusCode, string(body))
	}

	var r esearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("esearch decode: %w", err)
	}

	count, _ := strconv.Atoi(r.ESearchResult.Count)
	rm, _ := strconv.Atoi(r.ESearchResult.RetMax)
	rs, _ := strconv.Atoi(r.ESearchResult.RetStart)
	return &SearchResult{
		Count:    count,
		PMIDs:    r.ESearchResult.IDList,
		RetMax:   rm,
		RetStart: rs,
	}, nil
}
