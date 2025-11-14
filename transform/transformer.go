// Package transform defines format-agnostic abstractions for turning a raw
// byte stream into decoded records, and mapping those records into strongly
// typed Go structs.
//
// The core pipeline is:
//
//   connector.SrcAwareStreamer (bytes + source metadata)
//     → Decoder (produces RecordIterator of generic records)
//     → Mapper[T] (converts each record into a T)
//     → StructIterator[T] (stream of typed values)
//
// This separation lets you plug in different decoders (CSV, XML, JSON, …)
// and different mappers for each schema, while keeping the streaming model
// and source-awareness consistent across the project.

package transform

import (
	"context"

	"github.com/carlodf/cetl/connector"
)

//
// Access over one decoded record (format-agnostic)
//

// Extractor provides read-only access to a single decoded record.
//
// Implementations are format-specific (CSV, XML, JSON, …) but expose a
// common access pattern so mappers can be reused across decoders. A record
// is conceptually a flat list of fields with optional names.

type Extractor interface {
	// ByIndex returns the field value at index i and true if present.
	// Implementations must return ok == false for out-of-bounds indices.
	ByIndex(i int) (string, bool)

	// ByName returns the field value for the given name and true if present.
	// If the underlying format does not provide names (no header), ByName
	// must return ok == false.
	ByName(name string) (string, bool)

	// Len reports number of fields in the current record.
	Len() int

	// Names returns the field names for the current record if available, or
	// nil if the format has no header or the decoder is not name-aware.
	Names() []string

	// Meta returns the source metadata for the current record, such as the
	// originating file name and byte offset, as provided by the underlying
	// connector.SrcAwareStreamer.
	Meta() connector.SrcMeta
}

//
// Streaming iterators
//

// RecordIterator is a forward-only iterator over decoded records.
//
// The typical usage pattern is:
//
//	it, err := dec.Decode(ctx, stream)
//	if err != nil { ... }
//	defer it.Close()
//
//	for it.Next() {
//	    rec := it.Record()
//	    // use rec.ByIndex / rec.ByName / rec.Meta ...
//	}
//	if err := it.Err(); err != nil {
//	    // handle stream/decoder error
//	}
type RecordIterator interface {

	// Next advances to the next record and reports whether one is available.
	// It returns false on EOF or on a terminal error. When Next returns
	// false, Err must be checked to distinguish clean EOF from failure.
	Next() bool

	// Record returns the current record. It is only valid to call Record
	// after Next has returned true, and its result remains valid until the
	// next call to Next.
	Record() Extractor

	// Err returns the first non-EOF error encountered while iterating, or
	// nil if the iterator completed successfully.
	Err() error

	// Close releases any underlying resources. It must be safe to call
	// Close multiple times. Implementations should tolerate Close being
	// called before the iterator is fully exhausted.
	Close() error
}

// StructIterator is a forward-only iterator over strongly typed values
// produced by applying a Mapper to each decoded record.
type StructIterator[T any] interface {

	// Next advances to the next value and reports whether one is available.
	// It returns false on EOF or on a terminal error. When Next returns
	// false, Err must be checked to distinguish clean EOF from failure.
	Next() bool

	// Struct returns the current value of type T. It is only valid to call
	// Struct after Next has returned true, and its result remains valid
	// until the next call to Next.
	Struct() T

	// Err returns the first non-EOF error encountered while iterating, or
	// nil if the iterator completed successfully.
	Err() error

	// Close releases any underlying resources. It must be safe to call
	// Close multiple times. Implementations should tolerate Close being
	// called before the iterator is fully exhausted.
	Close() error
}

//
// Decoder for a specific serialization format
//

// Decoder turns a source-aware byte stream into a stream of decoded records.
//
// A Decoder is responsible for a specific on-wire format (e.g., CSV, XML).
// Any format-specific configuration (delimiter, header handling, namespaces,
// etc.) should be supplied when constructing the Decoder, not at Decode time.
type Decoder interface {

	// Decode consumes bytes from rc and produces a RecordIterator. The
	// returned iterator owns rc and is responsible for closing it when
	// iteration ends or Close is called.
	//
	// The rc parameter exposes both io.Reader and source metadata via
	// connector.SrcAwareStreamer, allowing the decoder to maintain
	// per-record provenance (file name, byte offset, source boundaries).
	Decode(ctx context.Context, rc connector.SrcAwareStreamer) (RecordIterator, error)
}

//
// Mapper from a record to your strongly-typed struct
//

// Mapper converts a single decoded record into a strongly typed value T.
//
// Mappers are typically small, schema-specific functions that pull fields
// out of the Extractor (by index and/or by name), perform validation and
// type conversion, and return either a T or an error.
type Mapper[T any] func(Extractor) (T, error)

//
// Transformer = Decoder + Mapper composed
//

// Transformer composes a Decoder with a Mapper to produce a stream of
// strongly-typed values T from a source-aware byte stream.
type Transformer[T any] interface {

	// Transform decodes records from rc using the embedded Decoder, applies
	// mapFn to each record to produce a T, and returns a StructIterator
	// over the resulting values.
	//
	// The returned iterator owns rc and is responsible for closing it when
	// iteration ends or Close is called.
	Transform(ctx context.Context, rc connector.SrcAwareStreamer, mapFn Mapper[T]) (StructIterator[T], error)
}
