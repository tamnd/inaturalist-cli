// Package inaturalist exposes iNaturalist data as a kit Domain.
// A multi-domain host (ant) enables this driver with a single blank import:
//
//	import _ "github.com/tamnd/inaturalist-cli/inaturalist"
//
// The same Domain also builds the standalone inaturalist binary.
package inaturalist

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the iNaturalist driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "inaturalist",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "inaturalist",
			Short:  "Browse iNaturalist nature observations from the terminal",
			Long: `Browse iNaturalist nature observations from the terminal.

inaturalist reads taxa, observations, species counts, and places from the iNaturalist
API (api.inaturalist.org) over plain HTTPS, shapes the data into clean records, and
prints output that pipes into the rest of your tools. No API key required.`,
			Site: "inaturalist.org",
			Repo: "https://github.com/tamnd/inaturalist-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "taxa", Group: "taxonomy", List: true,
		Summary: "Search taxa by name"}, listTaxa)

	kit.Handle(app, kit.OpMeta{Name: "taxon", Group: "taxonomy", Single: true,
		Summary: "Get a taxon by numeric iNaturalist ID",
		Args:    []kit.Arg{{Name: "id", Help: "iNaturalist taxon ID (integer)"}}}, getTaxon)

	kit.Handle(app, kit.OpMeta{Name: "observations", Group: "observation", List: true,
		Summary: "Search observation records"}, listObservations)

	kit.Handle(app, kit.OpMeta{Name: "species-counts", Group: "observation", List: true,
		Summary: "Get species count summary for a place"}, listSpeciesCounts)

	kit.Handle(app, kit.OpMeta{Name: "places", Group: "place", List: true,
		Summary: "Search places by name",
		Args:    []kit.Arg{{Name: "query", Help: "place name to search"}}}, listPlaces)
}

// newClient builds the Client from kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// ─── input structs ────────────────────────────────────────────────────────────

type taxaInput struct {
	Search string  `kit:"flag" help:"taxon name to search"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type taxonInput struct {
	ID     string  `kit:"arg" help:"iNaturalist taxon ID (integer)"`
	Client *Client `kit:"inject"`
}

type observationsInput struct {
	Taxon   string  `kit:"flag" help:"taxon name to filter by"`
	Place   int     `kit:"flag" help:"place ID to filter by"`
	Quality string  `kit:"flag" help:"quality grade: research, needs_id, casual"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type speciesCountsInput struct {
	Place  int     `kit:"flag" help:"place ID"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type placesInput struct {
	Query  string  `kit:"arg" help:"place name to search"`
	Client *Client `kit:"inject"`
}

// ─── handlers ─────────────────────────────────────────────────────────────────

func listTaxa(ctx context.Context, in taxaInput, emit func(*Taxon) error) error {
	items, err := in.Client.SearchTaxa(ctx, in.Search, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func getTaxon(ctx context.Context, in taxonInput, emit func(*Taxon) error) error {
	id, err := strconv.Atoi(in.ID)
	if err != nil {
		return errs.Usage("taxon id must be a number, got %q", in.ID)
	}
	item, err := in.Client.GetTaxon(ctx, id)
	if err != nil {
		return err
	}
	return emit(&item)
}

func listObservations(ctx context.Context, in observationsInput, emit func(*Observation) error) error {
	p := ObservationParams{
		TaxonName:    in.Taxon,
		PlaceID:      in.Place,
		QualityGrade: in.Quality,
		Limit:        in.Limit,
	}
	items, err := in.Client.SearchObservations(ctx, p)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func listSpeciesCounts(ctx context.Context, in speciesCountsInput, emit func(*PlaceCount) error) error {
	items, err := in.Client.SpeciesCounts(ctx, in.Place, in.Limit)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func listPlaces(ctx context.Context, in placesInput, emit func(*Place) error) error {
	items, err := in.Client.SearchPlaces(ctx, in.Query)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// ─── URI driver (pure string functions, network-free) ─────────────────────────

// Classify turns a taxon id or iNaturalist URL into (type, id).
func (Domain) Classify(input string) (string, string, error) {
	return "taxon", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	switch t {
	case "taxon":
		return fmt.Sprintf("https://www.inaturalist.org/taxa/%s", id), nil
	case "observation":
		return fmt.Sprintf("https://www.inaturalist.org/observations/%s", id), nil
	case "place":
		return fmt.Sprintf("https://www.inaturalist.org/places/%s", id), nil
	default:
		return "", errs.Usage("inaturalist has no resource type %q", t)
	}
}
