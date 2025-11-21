# CETL — Composable ETL Building Blocks in Go

CETL is a small set of orthogonal packages for building streaming ETL pipelines in Go. It focuses on composability, observability (source + byte offsets), and memory-efficient streaming.

```
+-----------+     +-------------+     +------------------+
|  opener   | --> |  connector  | --> |    transform     |
+-----------+     +-------------+     +------------------+
```

- opener: where bytes come from (files, in-memory for tests)
- connector: multiplex sources into one stream with source-awareness
- transform: decode bytes into records and map to typed structs


## Install

- Go 1.25+
- `go get github.com/carlodf/cetl@latest`

Packages are imported by path:
- `github.com/carlodf/cetl/opener`
- `github.com/carlodf/cetl/connector`
- `github.com/carlodf/cetl/transform`


## Quick Start

### Stream multiple files as one CSV/PSV row stream

This example reads all `|`-separated files under `logs/` and prints rows while preserving which file produced them.

```go
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"github.com/carlodf/cetl/connector"
	"github.com/carlodf/cetl/opener"
)

func main() {
	ctx := context.Background()

	ops, err := opener.RegularFileOpenerFactory("logs/*.psv")
	if err != nil { panic(err) }

	mux := connector.NewMuxReader(ctx, ops)
	defer mux.Close()

	r := csv.NewReader(mux)
	r.Comma = '|'

	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) { break }
		if err != nil { panic(err) }
		fmt.Println(mux.Current().Name, row)
	}
}
```

Example output:

```
logs/2024-10-01.psv [field1 field2 field3]
logs/2024-10-01.psv [field1 field2 field3]
logs/2024-10-02.psv [field1 field2 field3]
```

### Decode CSV with header handling and per-source boundaries

`transform.NewCSVDecoder` keeps a canonical header and automatically skips repeated headers at the start of each new source when the header is inferred.

```go
ctx := context.Background()
ops, _ := opener.RegularFileOpenerFactory("data/*.csv")
mux := connector.NewMuxReader(ctx, ops)

dec := transform.NewCSVDecoder(transform.CSVDecoderOptions{Comma: ','})
it, err := dec.Decode(ctx, mux)
if err != nil { panic(err) }

// Always close the iterator to release the underlying stream
defer it.Close()

for it.Next() {
	rec := it.Record()
	// Access by name or by index
	id, _ := rec.ByName("id")
	val0, _ := rec.ByIndex(0)
	fmt.Println(rec.Meta().Name, id, val0) // file name + fields
}
if err := it.Err(); err != nil { panic(err) }
```

### Map rows to your own struct

Use a decoder + mapper via `NewDecodeMapTransform[T]`.

```go
type Event struct {
	ID   string
	Name string
}

dec := transform.NewCSVDecoder(transform.CSVDecoderOptions{Comma: ','})
tr  := transform.NewDecodeMapTransform[Event](dec)

it, err := tr.Transform(ctx, mux, func(ex transform.Extractor) (Event, error) {
	id, _ := ex.ByName("id")
	name, _ := ex.ByName("name")
	return Event{ID: id, Name: name}, nil
})
if err != nil { panic(err) }
defer it.Close()

for it.Next() {
	e := it.Struct()
	fmt.Printf("%s: %+v\n", e.ID, e)
}
if err := it.Err(); err != nil { panic(err) }
```


## Package Overview

- opener
  - `type Opener`: `Open(ctx) (io.ReadCloser, error)` + `Name()`
  - `RegularFileOpenerFactory(spec string) ([]Opener, error)`: glob/URL/Windows-aware
  - `NewFile(path string) File`: lazy file opener
  - `InMemorySource{Data []byte, SourceName string}`: test helper

- connector
  - `NewMuxReader(ctx, ops []opener.Opener) SrcAwareStreamer`
  - Single stream over many sources; only one source open at a time
  - `Current() SrcMeta`: `{Name string, ByteOffset int64}`
  - `AwaitBoundary(ctx) (SrcMeta, error)`: blocks until next source starts; `io.EOF` when done

- transform
  - `Decoder` → `RecordIterator` of records with `ByName`, `ByIndex`, `Names`, `Meta`
  - `NewCSVDecoder(CSVDecoderOptions{Comma, Header})`
    - If `Header` empty: infer from first record, enforce across sources, skip repeated headers
  - `NewDecodeMapTransform[T](Decoder)` → `Transformer[T]` from bytes to typed values


## File Spec Support (opener.RegularFileOpenerFactory)

Accepted specs (normalized for `filepath.Glob`):
- Plain paths or globs: `/data/*.csv`, `logs/*.psv`
- File URLs (hierarchical): `file:///path/to/file.txt`
- File URLs (opaque): `file:/absolute/or/windows/path`
- Windows drive paths: `C:\path\to\file.txt`
- Windows UNC paths: `\\server\share\dir\file.txt`

Invalid or unsupported schemes (e.g. `http://`) return an error.


## Boundary Awareness

`connector.SrcAwareStreamer` exposes boundaries and offsets so you can correlate decoded records to their origin:

```go
// Before reading bytes from a source, a boundary is emitted.
meta, err := mux.AwaitBoundary(ctx) // meta.ByteOffset == 0

// While streaming, you can poll the current position.
cur := mux.Current() // {Name: "...", ByteOffset: N}
```

Semantics:
- Boundaries are coalesced (buffer=1): only the latest unseen boundary is retained.
- On read errors, partial bytes are delivered first; the error is then returned.
- After all sources: `Read` and `AwaitBoundary` return `io.EOF`.


## Development

- Build: `go build ./...`
- Format: `go fmt ./...` (check: `gofmt -s -l .`)
- Lint: `go vet ./...`
- Tests: `go test ./...` or `make test`
- Coverage: `make cover` (HTML: `make cover-html`, open: `make cover-view`)


## Status

Early-stage; APIs may evolve as more decoders and loaders are added.


## License

Apache 2.0 (see `LICENSE`).
