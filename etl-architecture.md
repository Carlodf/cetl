# ETL Scaffolding: Openers → Connectors → Transformers → Parser → Extractor

This document captures the agreed architecture and minimal interfaces so you can implement the system and evolve it later. It also shows **where to plug in an encoding/UTF‑8 converter** cleanly.

---

## High-level flow

1. **Openers** – provide a _byte stream_ (`io.ReadCloser`) and a stable **Name** (path/URL/logical id). No parsing, no metadata guessing.
2. **(Optional) Reader Middleware** – transparent `io.Reader` wrappers (e.g., **encoding/UTF‑8 conversion**, decompression, rate-limit, tracing). Lives **between Openers and Connectors**.
3. **Connectors** – consume one or many openers and emit **one merged stream** (`io.ReadCloser`). Example: `fileconcat` opens sources one-by-one and concatenates them.
4. **Transformers** – operate on **rows**, not bytes. They receive stable **context** (source name + per-source record index + byte offsets) and return transformed rows (e.g., append `file_date`, provenance, offsets).
5. **Parser** – wrap the connector’s stream with `encoding/csv.Reader` (comma `'|'`) to read PSV rows.
6. **Extractor** – map header+row to your typed structs or build an in-memory index for queries.

> Design principle: **layers do one thing**. Bytes concerns (encoding/compression) are handled before row concerns (CSV, schema).

---

## Go package layout (suggested)

```
tufano.com/etl/
  opener/                # adapters that can open a stream (filesystem, http, s3 later)
    opener.go
    file.go
    # future: http.go, s3.go, gzip.go (as decorators)
  connectors/
    fileconcat/
      fileconcat.go      # concatenates multiple openers → one stream
  transformers/          # reusable row transformers (append provenance, dates, etc.)
    append_origin.go
  parsers/               # (optional) helpers around encoding/csv
    csvutil.go
  extract/               # struct-mapping, indexing, querying
    scan.go
```

---

## Core interfaces & types

### opener

```go
// opener/opener.go
package opener

import (
	"context"
	"io"
)

type Opener interface {
	Open(ctx context.Context) (io.ReadCloser, error)
	Name() string // stable identity: filepath, URL, topic/partition, etc.
}
```

```go
// opener/file.go
package opener

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type File struct{ Path string }

func (f File) Open(context.Context) (io.ReadCloser, error) { return os.Open(f.Path) }
func (f File) Name() string                                { return filepath.Clean(f.Path) }
```

### (Optional) Reader middleware (encoding/UTF‑8 converter, gzip, etc.)

Implement as **decorators** that wrap an existing `Opener` and return a new `Opener`.

```go
// opener/utf8.go
package opener

import (
	"bufio"
	"context"
	"io"

	"golang.org/x/text/transform"
	"golang.org/x/text/encoding/charmap" // or detect with what you prefer
)

// UTF8 returns an Opener that decodes the underlying stream to UTF‑8.
func UTF8(o Opener, dec transform.Transformer) Opener {
	type utf8Opener struct{ Opener }
	return utf8Opener{o}
}

func (u utf8Opener) Open(ctx context.Context) (io.ReadCloser, error) {
	rc, err := u.Opener.Open(ctx)
	if err != nil { return nil, err }
	tr := transform.NewReader(rc, charmap.ISO8859_1.NewDecoder()) // example; plug your transformer
	br := bufio.NewReader(tr)

	// Tie Close() to the original rc
	return struct {
		io.Reader
		io.Closer
	}{Reader: br, Closer: rc}, nil
}
```

> You can stack multiple decorators: `opener.UTF8(opener.Gzip(opener.File{Path: ...}))`

### connectors/fileconcat

```go
// connectors/fileconcat/fileconcat.go
package fileconcat

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
)

type Opener interface {
	Open(ctx context.Context) (io.ReadCloser, error)
	Name() string
}

type SourceCtx struct {
	Name string
}

type RecordCtx struct {
	Source    SourceCtx
	Record    int64 // 0-based within this source (data rows only)
	ByteStart int64 // byte offset before reading the record
	ByteEnd   int64 // byte offset after reading the record
}

type Transformer interface {
	Header(in []string, src SourceCtx) ([]string, error)
	Record(in []string, rc RecordCtx) ([]string, error)
}

type countingReader struct{ r io.Reader; n int64 }
func (c *countingReader) Read(p []byte) (int, error) { k, err := c.r.Read(p); c.n += int64(k); return k, err }
func (c *countingReader) Offset() int64              { return c.n }

// New: open each source sequentially, transform header+rows, emit a single PSV stream.
func New(ctx context.Context, ops []Opener, t Transformer) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()

		var wroteHeader bool
		var unifiedHeader []string

		for _, o := range ops {
			rc, err := o.Open(ctx)
			if err != nil { _ = pw.CloseWithError(fmt.Errorf("open %s: %w", o.Name(), err)); return }

			cr := &countingReader{r: rc}
			r := csv.NewReader(cr)
			r.Comma = '|'
			r.FieldsPerRecord = -1 // allow variable columns; transformer sets final shape

			// header
			hdr, err := r.Read()
			if err != nil { rc.Close(); _ = pw.CloseWithError(fmt.Errorf("%s header: %w", o.Name(), err)); return }

			src := SourceCtx{Name: o.Name()}
			outHdr, err := t.Header(hdr, src)
			if err != nil { rc.Close(); _ = pw.CloseWithError(err); return }

			w := csv.NewWriter(pw)
			w.Comma = '|'

			if !wroteHeader {
				unifiedHeader = append([]string(nil), outHdr...)
				_ = w.Write(unifiedHeader)
				w.Flush()
				if err := w.Error(); err != nil { rc.Close(); _ = pw.CloseWithError(err); return }
				wroteHeader = true
			} else if len(outHdr) != len(unifiedHeader) {
				rc.Close(); _ = pw.CloseWithError(fmt.Errorf("%s: transformed header mismatch", o.Name())); return
			}

			// records
			var recIdx int64
			for {
				start := cr.Offset()
				row, err := r.Read()
				if err == io.EOF { break }
				if err != nil { rc.Close(); _ = pw.CloseWithError(fmt.Errorf("%s record: %w", o.Name(), err)); return }
				end := cr.Offset()

				rcx := RecordCtx{Source: src, Record: recIdx, ByteStart: start, ByteEnd: end}
				outRow, err := t.Record(row, rcx)
				if err != nil { rc.Close(); _ = pw.CloseWithError(err); return }

				_ = w.Write(outRow)
				w.Flush()
				if err := w.Error(); err != nil { rc.Close(); _ = pw.CloseWithError(err); return }

				recIdx++
			}
			_ = rc.Close()
		}
	}()
	return pr, nil
}
```

### Example transformer – append provenance

```go
// transformers/append_origin.go
package transformers

import (
	"fmt"

	"tufano.com/etl/connectors/fileconcat"
)

type AppendOrigin struct{}

func (AppendOrigin) Header(in []string, _ fileconcat.SourceCtx) ([]string, error) {
	out := append([]string{}, in...)
	return append(out, "source_name", "record_index"), nil
}

func (AppendOrigin) Record(in []string, rc fileconcat.RecordCtx) ([]string, error) {
	out := append([]string{}, in...)
	return append(out, rc.Source.Name, fmt.Sprintf("%d", rc.Record)), nil
}
```

---

## Parser and Extractor (usage)

```go
ctx := context.Background()

ops := []fileconcat.Opener{
    opener.File{Path: "data/invoices_2025-09-30.psv"},
    opener.File{Path: "data/invoices_2025-10-01.psv"},
}
// Optional: force UTF‑8 decoding
// ops[0] = opener.UTF8(ops[0], charmap.ISO8859_1.NewDecoder())

rc, err := fileconcat.New(ctx, ops, transformers.AppendOrigin{})
if err != nil { log.Fatal(err) }
defer rc.Close()

cr := csv.NewReader(rc)
cr.Comma = '|'
cr.FieldsPerRecord = 0 // enforce the transformed header shape

header, err := cr.Read()
if err != nil { log.Fatal(err) }

for {
    rec, err := cr.Read()
    if err == io.EOF { break }
    if err != nil { log.Fatal(err) }
    // feed to your extractor / struct mapper
}
```

---

## Why the UTF‑8 converter belongs before the connector

- **Encoding is a byte-level concern**; transformers work on **rows** after parsing.
- By placing decoding in an **Opener decorator**, all downstream code sees clean UTF‑8 bytes.
- You can stack multiple reader middlewares (gzip→utf8→trace) without touching connectors or transformers.

---

## Testing tips

- Unit test **opener.File** with testdata files.
- Unit test **UTF‑8 decorator** with a small ISO‑8859‑1 fixture.
- Unit test **fileconcat** with two tiny PSV files; assert:
  - header written once,
  - provenance columns,
  - `RecordCtx.Record` increments,
  - byte offsets are monotonic.
- Property tests for transformers: input length ↔ output length, column invariants.

---

## Makefile (Linux)

```makefile
PKGS       = ./...
COVER_OUT  = cover.out
COVER_HTML = cover.html

.PHONY: test cover cover-html cover-view clean

test:
	go test $(PKGS)

cover:
	go test $(PKGS) -coverprofile=$(COVER_OUT)

cover-html: cover
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

cover-view: cover-html
	xdg-open $(COVER_HTML)

clean:
	rm -f $(COVER_OUT) $(COVER_HTML)
```
