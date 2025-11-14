package opener

import (
	"bytes"
	"context"
	"io"
)

// InMemorySource implements Opener using an in-memory byte slice.
//
// It is intended mainly for **testing** and **synthetic ETL pipelines**,
// where constructing temporary files would be unnecessary or inconvenient.
// InMemorySource allows you to:
//
//   - feed small datasets directly into the connector/multiplexer
//   - test multi-source boundary behavior deterministically
//   - benchmark decoders without touching the filesystem
//   - exercise transformation logic with predictable input
//
// Example usage:
//
//	srcs := []opener.Opener{
//	    opener.InMemorySource{
//	        Name: "sourceA",
//	        Data: []byte("a,b,c\n1,2,3\n"),
//	    },
//	    opener.InMemorySource{
//	        Name: "sourceB",
//	        Data: []byte("a,b,c\n4,5,6\n"),
//	    },
//	}
//
//	mux := connectors.NewMuxReader(ctx, srcs)
//	defer mux.Close()
//
//	dec := transform.NewCSVDecoder(transform.CSVDecoderOptions{})
//	it, _ := dec.Decode(ctx, mux)
//	defer it.Close()
//
//	for it.Next() {
//	    rec := it.Record()
//	    fmt.Println(rec.Meta().Name, rec.ByIndex(0))
//	}
//
// Production code should prefer real filesystem or network-backed Opener
// implementations. InMemorySource exists mainly to simplify tests and is
// not optimized for very large datasets.
type InMemorySource struct {
	// Data contains the bytes to be returned by Open().
	Data []byte
	// Name identifies the synthetic source. The multiplexer uses this as
	// the source name when emitting SrcMeta.
	SourceName string
}

// Open returns an io.ReadCloser that streams the in-memory data.
// The returned reader is independent of the InMemorySource’s buffer
// and may be safely closed by the caller.
//
// Always returns a non-nil ReadCloser and a nil error.
func (s InMemorySource) Open(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.Data)), nil
}

// Name returns the source identifier associated with this in-memory stream.
//
// This satisfies the opener.Opener interface and allows InMemorySource to
// be mixed transparently with file-based and network-based openers when
// constructing a connector multiplexer.
func (s InMemorySource) Name() string { // or Name(), depending on your interface
	return s.SourceName
}
