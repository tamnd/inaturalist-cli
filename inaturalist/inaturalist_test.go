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
	_, err := c.SearchTaxa(context.Background(), "test", "", 1)
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
	_, err := c.SearchTaxa(context.Background(), "test", "", 1)
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
				"id": 48662,
				"name": "Danaus plexippus",
				"rank": "species",
				"preferred_common_name": "Monarch Butterfly",
				"wikipedia_url": "https://en.wikipedia.org/wiki/Monarch_butterfly",
				"observations_count": 479446
			},
			{
				"id": 48663,
				"name": "Danaus plexippus plexippus",
				"rank": "subspecies",
				"preferred_common_name": "Monarch",
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
	taxa, err := c.SearchTaxa(context.Background(), "monarch butterfly", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(taxa) != 2 {
		t.Fatalf("got %d taxa, want 2", len(taxa))
	}
	if taxa[0].ID != 48662 {
		t.Errorf("ID = %d, want 48662", taxa[0].ID)
	}
	if taxa[0].Name != "Danaus plexippus" {
		t.Errorf("Name = %q, want Danaus plexippus", taxa[0].Name)
	}
	if taxa[0].Rank != "species" {
		t.Errorf("Rank = %q, want species", taxa[0].Rank)
	}
	if taxa[0].CommonName != "Monarch Butterfly" {
		t.Errorf("CommonName = %q, want Monarch Butterfly", taxa[0].CommonName)
	}
	if taxa[0].Observations != 479446 {
		t.Errorf("Observations = %d, want 479446", taxa[0].Observations)
	}
}

func TestSearchTaxaWithRankFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rank") != "species" {
			t.Errorf("rank = %q, want species", r.URL.Query().Get("rank"))
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchTaxa(context.Background(), "monarch", "species", 5)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetTaxonParsesFields(t *testing.T) {
	const body = `{
		"results": [
			{
				"id": 48662,
				"name": "Danaus plexippus",
				"rank": "species",
				"preferred_common_name": "Monarch Butterfly",
				"wikipedia_url": "https://en.wikipedia.org/wiki/Monarch_butterfly",
				"observations_count": 479446
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/taxa/48662" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	taxon, err := c.GetTaxon(context.Background(), 48662)
	if err != nil {
		t.Fatal(err)
	}
	if taxon.ID != 48662 {
		t.Errorf("ID = %d, want 48662", taxon.ID)
	}
	if taxon.Name != "Danaus plexippus" {
		t.Errorf("Name = %q, want Danaus plexippus", taxon.Name)
	}
	if taxon.CommonName != "Monarch Butterfly" {
		t.Errorf("CommonName = %q, want Monarch Butterfly", taxon.CommonName)
	}
	if taxon.WikipediaURL != "https://en.wikipedia.org/wiki/Monarch_butterfly" {
		t.Errorf("WikipediaURL = %q", taxon.WikipediaURL)
	}
	if taxon.Observations != 479446 {
		t.Errorf("Observations = %d, want 479446", taxon.Observations)
	}
}

func TestSearchObservationsAllFields(t *testing.T) {
	const body = `{
		"total_results": 23913,
		"results": [
			{
				"id": 123456,
				"species_guess": "Monarch Butterfly",
				"place_guess": "San Francisco, CA",
				"quality_grade": "research",
				"observed_on": "2024-01-15",
				"taxon": {
					"id": 48662,
					"name": "Danaus plexippus",
					"preferred_common_name": "Monarch Butterfly"
				},
				"user": {"login": "naturalist123"},
				"photos": [{"url": "https://inaturalist-open-data.s3.amazonaws.com/photos/123/square.jpg"}]
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observations" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("taxon_name") != "monarch butterfly" {
			t.Errorf("taxon_name = %q, want monarch butterfly", q.Get("taxon_name"))
		}
		if q.Get("quality_grade") != "research" {
			t.Errorf("quality_grade = %q, want research", q.Get("quality_grade"))
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{
		TaxonName:    "monarch butterfly",
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
	if o.Species != "Monarch Butterfly" {
		t.Errorf("Species = %q, want Monarch Butterfly", o.Species)
	}
	if o.ScientificName != "Danaus plexippus" {
		t.Errorf("ScientificName = %q, want Danaus plexippus", o.ScientificName)
	}
	if o.Place != "San Francisco, CA" {
		t.Errorf("Place = %q, want San Francisco, CA", o.Place)
	}
	if o.Observer != "naturalist123" {
		t.Errorf("Observer = %q, want naturalist123", o.Observer)
	}
	if o.Quality != "research" {
		t.Errorf("Quality = %q, want research", o.Quality)
	}
	if o.ObservedOn != "2024-01-15" {
		t.Errorf("ObservedOn = %q, want 2024-01-15", o.ObservedOn)
	}
	// photo URL should have square replaced with medium
	if o.PhotoURL != "https://inaturalist-open-data.s3.amazonaws.com/photos/123/medium.jpg" {
		t.Errorf("PhotoURL = %q", o.PhotoURL)
	}
}

func TestSearchObservationsTaxonID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("taxon_id") != "48662" {
			t.Errorf("taxon_id = %q, want 48662", q.Get("taxon_id"))
		}
		if q.Get("taxon_name") != "" {
			t.Errorf("taxon_name should be empty when taxon_id is set, got %q", q.Get("taxon_name"))
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{
		TaxonID: 48662,
		Limit:   5,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSearchObservationsPhotosParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("photos") != "true" {
			t.Errorf("photos param not set")
		}
		_, _ = w.Write([]byte(`{"total_results":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{
		Photos: true,
		Limit:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSearchObservationsFallbackSpeciesGuess(t *testing.T) {
	const body = `{
		"results": [
			{
				"id": 999,
				"species_guess": "Some Bird",
				"quality_grade": "needs_id",
				"observed_on": "2024-06-01"
			}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.SearchObservations(context.Background(), inaturalist.ObservationParams{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one item")
	}
	// without a taxon object, falls back to species_guess
	if items[0].Species != "Some Bird" {
		t.Errorf("Species = %q, want Some Bird", items[0].Species)
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
		t.Errorf("Name = %q", counts[0].Name)
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
	if places[0].DisplayName != "Nairobi National Park, Nairobi, Kenya" {
		t.Errorf("DisplayName = %q", places[0].DisplayName)
	}
}
