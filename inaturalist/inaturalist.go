// Package inaturalist is the library behind the inaturalist command line:
// the HTTP client, request shaping, wire decoding, and typed data models
// for the iNaturalist REST API.
//
// The API at https://api.inaturalist.org/v1 is entirely public and requires no
// authentication key for read-only access. The Client paces requests, sets a
// real User-Agent, and retries transient failures (429 and 5xx).
package inaturalist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to the iNaturalist API.
const DefaultUserAgent = "inaturalist-cli/0.1 (tamnd87@gmail.com)"

// Host is the iNaturalist API hostname.
const Host = "api.inaturalist.org"

// BaseURL is the root every API request is built from.
const BaseURL = "https://" + Host + "/v1"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to the iNaturalist REST API.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client built from cfg.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = BaseURL
	}
	cfg.BaseURL = base
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Observation is a single nature observation record from iNaturalist.
type Observation struct {
	ID             int    `kit:"id" json:"id"`
	ObservedOn     string `json:"observed_on"`
	Species        string `json:"species"`         // common name (taxon.preferred_common_name) or scientific name
	ScientificName string `json:"scientific_name"` // taxon.name
	Place          string `json:"place"`           // place_guess
	Observer       string `json:"observer"`        // user.login
	Quality        string `json:"quality"`         // quality_grade
	PhotoURL       string `json:"photo_url"`       // photos[0].url with small replaced by medium
}

// Taxon represents a species or higher taxonomic group from iNaturalist.
type Taxon struct {
	ID           int    `kit:"id" json:"id"`
	Name         string `json:"name"`          // scientific name
	CommonName   string `json:"common_name"`   // preferred_common_name
	Rank         string `json:"rank"`
	Observations int    `json:"observations"`  // observations_count
	WikipediaURL string `json:"wikipedia_url"`
}

// Place is a geographic place from the places autocomplete endpoint.
type Place struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// PlaceCount is an entry from the species_counts endpoint.
type PlaceCount struct {
	Count      int    `json:"count"`
	Name       string `json:"name"`
	CommonName string `json:"common_name"`
	Rank       string `json:"rank"`
}

// ObservationParams holds optional filters for observation searches.
type ObservationParams struct {
	Query        string
	TaxonName    string
	TaxonID      int
	PlaceName    string
	QualityGrade string
	Photos       bool
	Limit        int
}

// ─── wire types ──────────────────────────────────────────────────────────────

type taxaListResp struct {
	TotalResults int         `json:"total_results"`
	Results      []wireTaxon `json:"results"`
}

type taxonResp struct {
	Results []wireTaxon `json:"results"`
}

type wireTaxon struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Rank                string `json:"rank"`
	PreferredCommonName string `json:"preferred_common_name"`
	WikipediaURL        string `json:"wikipedia_url"`
	ObservationsCount   int    `json:"observations_count"`
}

type obsListResp struct {
	TotalResults int               `json:"total_results"`
	Results      []wireObservation `json:"results"`
}

type wireObservation struct {
	ID           int          `json:"id"`
	SpeciesGuess string       `json:"species_guess"`
	PlaceGuess   string       `json:"place_guess"`
	QualityGrade string       `json:"quality_grade"`
	ObservedOn   string       `json:"observed_on"`
	Taxon        *wireTaxon   `json:"taxon"`
	User         wireUser     `json:"user"`
	Photos       []wirePhoto  `json:"photos"`
}

type wireUser struct {
	Login string `json:"login"`
}

type wirePhoto struct {
	URL string `json:"url"`
}

type speciesCountsResp struct {
	Results []wireSpeciesCount `json:"results"`
}

type wireSpeciesCount struct {
	Count int            `json:"count"`
	Taxon wireCountTaxon `json:"taxon"`
}

type wireCountTaxon struct {
	Name                string `json:"name"`
	PreferredCommonName string `json:"preferred_common_name"`
	Rank                string `json:"rank"`
}

type placesResp struct {
	Results []wirePlace `json:"results"`
}

type wirePlace struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ─── mapping helpers ──────────────────────────────────────────────────────────

func wireTaxonToTaxon(w wireTaxon) Taxon {
	return Taxon{
		ID:           w.ID,
		Name:         w.Name,
		Rank:         w.Rank,
		CommonName:   w.PreferredCommonName,
		WikipediaURL: w.WikipediaURL,
		Observations: w.ObservationsCount,
	}
}

func wireObsToObservation(w wireObservation) Observation {
	o := Observation{
		ID:         w.ID,
		ObservedOn: w.ObservedOn,
		Place:      w.PlaceGuess,
		Observer:   w.User.Login,
		Quality:    w.QualityGrade,
	}
	if w.Taxon != nil {
		o.ScientificName = w.Taxon.Name
		if w.Taxon.PreferredCommonName != "" {
			o.Species = w.Taxon.PreferredCommonName
		} else {
			o.Species = w.Taxon.Name
		}
	} else if w.SpeciesGuess != "" {
		o.Species = w.SpeciesGuess
	}
	if len(w.Photos) > 0 {
		rawURL := w.Photos[0].URL
		// The API returns thumbnail URLs using size suffixes like /square, /small, /thumb.
		// We upgrade to /medium for a better-resolution default.
		switch {
		case strings.Contains(rawURL, "/square"):
			o.PhotoURL = strings.Replace(rawURL, "/square", "/medium", 1)
		case strings.Contains(rawURL, "/small"):
			o.PhotoURL = strings.Replace(rawURL, "/small", "/medium", 1)
		case strings.Contains(rawURL, "/thumb"):
			o.PhotoURL = strings.Replace(rawURL, "/thumb", "/medium", 1)
		default:
			o.PhotoURL = rawURL
		}
	}
	return o
}

func wireCountToPlaceCount(w wireSpeciesCount) PlaceCount {
	return PlaceCount{
		Count:      w.Count,
		Name:       w.Taxon.Name,
		CommonName: w.Taxon.PreferredCommonName,
		Rank:       w.Taxon.Rank,
	}
}

func wirePlaceToPlace(w wirePlace) Place {
	return Place{
		ID:          w.ID,
		Name:        w.Name,
		DisplayName: w.DisplayName,
	}
}

// ─── public methods ───────────────────────────────────────────────────────────

// SearchTaxa searches the /taxa endpoint by name and optional rank.
func (c *Client) SearchTaxa(ctx context.Context, query, rank string, limit int) ([]Taxon, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("per_page", strconv.Itoa(limit))
	if rank != "" {
		params.Set("rank", rank)
	}
	rawURL := c.cfg.BaseURL + "/taxa?" + params.Encode()

	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp taxaListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode taxa search: %w", err)
	}
	out := make([]Taxon, len(resp.Results))
	for i, w := range resp.Results {
		out[i] = wireTaxonToTaxon(w)
	}
	return out, nil
}

// GetTaxon fetches a single taxon by its numeric iNaturalist ID.
func (c *Client) GetTaxon(ctx context.Context, id int) (Taxon, error) {
	rawURL := c.cfg.BaseURL + "/taxa/" + strconv.Itoa(id)
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return Taxon{}, err
	}
	var resp taxonResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return Taxon{}, fmt.Errorf("decode taxon %d: %w", id, err)
	}
	if len(resp.Results) == 0 {
		return Taxon{}, fmt.Errorf("taxon %d not found", id)
	}
	return wireTaxonToTaxon(resp.Results[0]), nil
}

// SearchObservations searches observation records with optional filters.
func (c *Client) SearchObservations(ctx context.Context, p ObservationParams) ([]Observation, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.QualityGrade == "" {
		p.QualityGrade = "research"
	}
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(p.Limit))
	params.Set("quality_grade", p.QualityGrade)
	if p.Query != "" {
		params.Set("q", p.Query)
	}
	if p.TaxonID > 0 {
		params.Set("taxon_id", strconv.Itoa(p.TaxonID))
	} else if p.TaxonName != "" {
		params.Set("taxon_name", p.TaxonName)
	}
	if p.PlaceName != "" {
		params.Set("place_name", p.PlaceName)
	}
	if p.Photos {
		params.Set("photos", "true")
	}
	rawURL := c.cfg.BaseURL + "/observations?" + params.Encode()

	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp obsListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode observations: %w", err)
	}
	out := make([]Observation, len(resp.Results))
	for i, w := range resp.Results {
		out[i] = wireObsToObservation(w)
	}
	return out, nil
}

// SpeciesCounts returns species count summaries, optionally for a place.
func (c *Client) SpeciesCounts(ctx context.Context, placeID, limit int) ([]PlaceCount, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(limit))
	if placeID > 0 {
		params.Set("place_id", strconv.Itoa(placeID))
	}
	rawURL := c.cfg.BaseURL + "/observations/species_counts?" + params.Encode()

	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp speciesCountsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode species counts: %w", err)
	}
	out := make([]PlaceCount, len(resp.Results))
	for i, w := range resp.Results {
		out[i] = wireCountToPlaceCount(w)
	}
	return out, nil
}

// SearchPlaces searches places by name using the autocomplete endpoint.
func (c *Client) SearchPlaces(ctx context.Context, query string) ([]Place, error) {
	params := url.Values{}
	params.Set("q", query)
	rawURL := c.cfg.BaseURL + "/places/autocomplete?" + params.Encode()

	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp placesResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode places: %w", err)
	}
	out := make([]Place, len(resp.Results))
	for i, w := range resp.Results {
		out[i] = wirePlaceToPlace(w)
	}
	return out, nil
}

// ─── HTTP plumbing ────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if w := c.cfg.Rate - time.Since(c.last); w > 0 {
		time.Sleep(w)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
