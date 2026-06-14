package inaturalist_test

import (
	"testing"

	"github.com/tamnd/inaturalist-cli/inaturalist"
)

// These tests are offline: they exercise the URI driver's pure string functions.

func TestDomainInfo(t *testing.T) {
	info := inaturalist.Domain{}.Info()
	if info.Scheme != "inaturalist" {
		t.Errorf("Scheme = %q, want inaturalist", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != inaturalist.Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, inaturalist.Host)
	}
	if info.Identity.Binary != "inaturalist" {
		t.Errorf("Identity.Binary = %q, want inaturalist", info.Identity.Binary)
	}
}

func TestClassifyTaxon(t *testing.T) {
	typ, id, err := inaturalist.Domain{}.Classify("48662")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "taxon" {
		t.Errorf("type = %q, want taxon", typ)
	}
	if id != "48662" {
		t.Errorf("id = %q, want 48662", id)
	}
}

func TestLocateTaxon(t *testing.T) {
	got, err := inaturalist.Domain{}.Locate("taxon", "48662")
	want := "https://www.inaturalist.org/taxa/48662"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateObservation(t *testing.T) {
	got, err := inaturalist.Domain{}.Locate("observation", "123456")
	want := "https://www.inaturalist.org/observations/123456"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocatePlace(t *testing.T) {
	got, err := inaturalist.Domain{}.Locate("place", "6903")
	want := "https://www.inaturalist.org/places/6903"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := inaturalist.Domain{}.Locate("bogus", "123")
	if err == nil {
		t.Error("expected error for unknown resource type")
	}
}
