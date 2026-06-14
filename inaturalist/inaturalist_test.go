package inaturalist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/inaturalist-cli/inaturalist"
)

func newTestClient(ts *httptest.Server) *inaturalist.Client {
	cfg := inaturalist.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return inaturalist.NewClient(cfg)
}

func TestClientUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchTaxa(context.Background(), "test", 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	cfg := inaturalist.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := inaturalist.NewClient(cfg)

	start := time.Now()
	_, err := c.SearchTaxa(context.Background(), "test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestSearchTaxaReturnsResults(t *testing.T) {
	const body = `{
		"total_results": 9,
		"results": [
			{
				"id": 41869,
				"name": "Panthera leo",
				"rank": "species",
				"preferred_common_name": "lion",
				"wikipedia_url": "https://en.wikipedia.org/wiki/Panthera_leo",
				"observations_count": 24578
			},
			{
				"id": 41870,
				"name": "Panthera leo persica",
				"rank": "subspecies",
				"preferred_common_name": "Asiatic Lion",
				"observations_count": 312
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/taxa" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("q") == "" {
			t.Error("missing q param")
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	taxa, err := c.SearchTaxa(context.Background(), "Panthera leo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(taxa) != 2 {
		t.Fatalf("got %d taxa, want 2", len(taxa))
	}
	if taxa[0].ID != 41869 {
		t.Errorf("ID = %d, want 41869", taxa[0].ID)
	}
	if taxa[0].Name != "Panthera leo" {
		t.Errorf("Name = %q, want Panthera leo", taxa[0].Name)
	}
	if taxa[0].Rank != "species" {
		t.Errorf("Rank = %q, want species", taxa[0].Rank)
	}
	if taxa[0].CommonName != "lion" {
		t.Errorf("CommonName = %q, want lion", taxa[0].CommonName)
	}
	if taxa[0].ObservationsCount != 24578 {
		t.Errorf("ObservationsCount = %d, want 24578", taxa[0].ObservationsCount)
	}
	if taxa[0].URL != "https://www.inaturalist.org/taxa/41869" {
		t.Errorf("URL = %q", taxa[0].URL)
	}
}

func TestGetTaxonParsesFields(t *testing.T) {
	const body = `{
		"results": [
			{
				"id": 41869,
				"name": "Panthera leo",
				"rank": "species",
				"preferred_common_name": "lion",
				"wikipedia_url": "https://en.wikipedia.org/wiki/Panthera_leo",
				"observations_count": 24578
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/taxa/41869" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	taxon, err := c.GetTaxon(context.Background(), 41869)
	if err != nil {
		t.Fatal(err)
	}
	if taxon.ID != 41869 {
		t.Errorf("ID = %d, want 41869", taxon.ID)
	}
	if taxon.WikipediaURL != "https://en.wikipedia.org/wiki/Panthera_leo" {
		t.Errorf("WikipediaURL = %q", taxon.WikipediaURL)
	}
	if taxon.CommonName != "lion" {
		t.Errorf("CommonName = %q, want lion", taxon.CommonName)
	}
}

func TestSearchObservationsWithParams(t *testing.T) {
	const body = `{
		"total_results": 23913,
		"results": [
			{
				"id": 123456,
				"species_guess": "Lion",
				"place_guess": "Narok, KE",
				"quality_grade": "research",
				"observed_on": "2024-01-15"
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observations" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("taxon_name") != "Panthera leo" {
			t.Errorf("taxon_name = %q, want Panthera leo", q.Get("taxon_name"))
		}
		if q.Get("quality_grade") != "research" {
			t.Errorf("quality_grade = %q, want research", q.Get("quality_grade"))
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{
		TaxonName:    "Panthera leo",
		QualityGrade: "research",
		Limit:        5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	o := items[0]
	if o.ID != 123456 {
		t.Errorf("ID = %d, want 123456", o.ID)
	}
	if o.SpeciesGuess != "Lion" {
		t.Errorf("SpeciesGuess = %q, want Lion", o.SpeciesGuess)
	}
	if o.PlaceGuess != "Narok, KE" {
		t.Errorf("PlaceGuess = %q", o.PlaceGuess)
	}
	if o.QualityGrade != "research" {
		t.Errorf("QualityGrade = %q, want research", o.QualityGrade)
	}
	if o.ObservedOn != "2024-01-15" {
		t.Errorf("ObservedOn = %q, want 2024-01-15", o.ObservedOn)
	}
	if o.URL != "https://www.inaturalist.org/observations/123456" {
		t.Errorf("URL = %q", o.URL)
	}
}

func TestSpeciesCountsParsesNested(t *testing.T) {
	const body = `{
		"results": [
			{
				"count": 234,
				"taxon": {
					"name": "Crocuta crocuta",
					"preferred_common_name": "Spotted Hyena",
					"rank": "species"
				}
			},
			{
				"count": 189,
				"taxon": {
					"name": "Equus quagga",
					"preferred_common_name": "Plains Zebra",
					"rank": "species"
				}
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observations/species_counts" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("place_id") != "6903" {
			t.Errorf("place_id = %q, want 6903", r.URL.Query().Get("place_id"))
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	counts, err := c.SpeciesCounts(context.Background(), 6903, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 {
		t.Fatalf("got %d counts, want 2", len(counts))
	}
	if counts[0].Count != 234 {
		t.Errorf("Count = %d, want 234", counts[0].Count)
	}
	if counts[0].Name != "Crocuta crocuta" {
		t.Errorf("Name = %q, want Crocuta crocuta", counts[0].Name)
	}
	if counts[0].CommonName != "Spotted Hyena" {
		t.Errorf("CommonName = %q, want Spotted Hyena", counts[0].CommonName)
	}
	if counts[0].Rank != "species" {
		t.Errorf("Rank = %q, want species", counts[0].Rank)
	}
}

func TestSearchPlaces(t *testing.T) {
	const body = `{
		"results": [
			{
				"id": 6903,
				"name": "Nairobi",
				"display_name": "Nairobi National Park, Nairobi, Kenya"
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/places/autocomplete" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("q") == "" {
			t.Error("missing q param")
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	places, err := c.SearchPlaces(context.Background(), "Nairobi")
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].ID != 6903 {
		t.Errorf("ID = %d, want 6903", places[0].ID)
	}
	if places[0].Name != "Nairobi" {
		t.Errorf("Name = %q, want Nairobi", places[0].Name)
	}
	if places[0].DisplayName != "Nairobi National Park, Nairobi, Kenya" {
		t.Errorf("DisplayName = %q", places[0].DisplayName)
	}
}

func TestObservationParamsAllEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("taxon_name") != "" {
			t.Errorf("unexpected taxon_name param")
		}
		if q.Get("place_id") != "" {
			t.Errorf("unexpected place_id param")
		}
		if q.Get("quality_grade") != "" {
			t.Errorf("unexpected quality_grade param")
		}
		if q.Get("per_page") == "" {
			t.Error("missing per_page param")
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{})
	if err != nil {
		t.Fatal(err)
	}
}
