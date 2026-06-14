# inaturalist

Browse iNaturalist nature observations from the terminal

`inaturalist` is a single pure-Go binary. It reads public inaturalist data
over plain HTTPS, shapes it into clean records, and prints output that pipes
into the rest of your tools. No API key, nothing to run alongside it.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
inaturalist as `inaturalist://` URIs.

## Install

```bash
go install github.com/tamnd/inaturalist-cli/cmd/inaturalist@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/inaturalist-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/inaturalist:latest --help
```

## Usage

```bash
inaturalist page <path>                      # fetch one page as a record
inaturalist page <path> -o json              # as JSON, ready for jq
inaturalist page <path> --template '{{.Body}}'  # just the readable body text
inaturalist links <path>                     # the pages it links to, one per line
inaturalist --help                           # the whole command tree
```

Every command shares one output contract: `-o table|json|jsonl|csv|tsv|url|raw`,
`--fields` to pick columns, `--template` for a custom line, and `-n` to limit.
The default adapts to where output goes (a table on a terminal, JSONL in a
pipe), so the same command reads well by hand and parses cleanly downstream.

This is a fresh scaffold. It ships one example resource type, `page`, wired end
to end. Model the real inaturalist records in `inaturalist/` and declare their
operations in `inaturalist/domain.go`; each one becomes a command, an HTTP
route, and an MCP tool at once.

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
inaturalist serve --addr :7777    # GET /v1/page/<path>  returns NDJSON
inaturalist mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`inaturalist` registers a `inaturalist` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/inaturalist-cli/inaturalist"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `inaturalist://` URIs without knowing anything about inaturalist:

```bash
ant get inaturalist://page/<path>   # fetch the record
ant cat inaturalist://page/<path>   # just the body text
ant ls  inaturalist://page/<path>   # the pages it links to, each addressable
ant url inaturalist://page/<path>   # the live https URL
```

## Development

```
cmd/inaturalist/   thin main: hands cli.NewApp to kit.Run
cli/                 assembles the kit App from the inaturalist domain
inaturalist/                the library: HTTP client, data models, and domain.go (the driver)
docs/                tago documentation site
```

```bash
make build      # ./bin/inaturalist
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
