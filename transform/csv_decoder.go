package transform

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"github.com/carlodf/cetl/connector"
)

//
// Public API
//

// CSVDecoderOptions configures how a CSVDecoder parses input.
//
// If Header is non-nil and non-empty, it is used as the canonical header
// and every record must match its length. If Header is nil or empty,
// the decoder will read the first CSV record from the stream and treat
// it as the header.
//
// Comma controls the field delimiter. If Comma is zero, ',' is used.
type CSVDecoderOptions struct {
	Comma  rune
	Header []string
}

// NewCSVDecoder constructs a CSV-specific Decoder.
//
// The returned Decoder produces RecordIterator values backed by
// encoding/csv.Reader and understands a header row:
//
//   - If opt.Header is non-empty, that slice is used as the header.
//   - Otherwise, the header is inferred from the first record in the stream.
//
// When used together with a connector.SrcAwareStreamer that concatenates
// multiple CSV sources, NewCSVDecoder will:
//
//   - Enforce a single canonical header for all sources.
//   - Automatically drop a repeated header row at the start of each
//     new source when the inferred header matches.
//
// Input is expected to follow the CSV rules described in RFC 4180
// (with a configurable delimiter).
func NewCSVDecoder(opt CSVDecoderOptions) Decoder {
	optComma := ','
	optHeader := []string{}
	if opt.Comma != 0 {
		optComma = opt.Comma
	}
	if opt.Header != nil {
		optHeader = opt.Header
	}
	decoder := &csvDecoder{
		comma:  optComma,
		header: optHeader,
	}
	return decoder
}

// Decode consumes a connector.SrcAwareStreamer and returns a RecordIterator
// that yields one CSV record at a time.
//
// Header handling:
//
//   - If the decoder was configured with an explicit Header, every record
//     must have the same number of fields as that header.
//   - If no Header was configured, Decode reads the first CSV record
//     and uses it as the header.
//
// Multi-source handling:
//
//   - The provided SrcAwareStreamer is typically a stream that concatenates
//     multiple underlying sources (files).
//   - csvRowIterator uses SrcMeta from rc.Current() to detect when a new
//     source begins.
//   - When the header was inferred, the first record of each new source
//     is compared with the canonical header; if it matches, that record
//     is treated as a source-local header and skipped.
//
// The returned iterator is not safe for concurrent use. Call Close on the
// RecordIterator when you are done to release the underlying stream.
func (d *csvDecoder) Decode(ctx context.Context, rc connector.SrcAwareStreamer) (RecordIterator, error) {
	csvReader := csv.NewReader(rc)
	csvReader.Comma = d.comma
	csvReader.ReuseRecord = false
	csvReader.TrimLeadingSpace = true
	var csvHeader []string
	csvHeaderInferred := len(d.header) == 0
	if csvHeaderInferred {
		firstRec, err := csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("unable to infer header from first record: %w", err)
		}
		csvHeader = append(csvHeader, firstRec...)
	} else {
		csvHeader = append(csvHeader, d.header...)
	}
	csvReader.FieldsPerRecord = len(csvHeader)
	if err := validateHeader(csvHeader); err != nil {
		return nil, fmt.Errorf("malformed header: %w", err)
	}
	it := &csvRowIterator{
		csvReader:      csvReader,
		srcAwareStream: rc,
		header:         csvHeader,
		atStart:        false,
		invertedIndex:  buildIndex(csvHeader),
		decoderError:   nil,
		current:        make([]string, len(csvHeader)),
		lastSourceMeta: rc.Current(),
	}
	// Best-effort: close the underlying stream if the context is cancelled.
	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()
	return it, nil
}

// Iterator implementation
func (it *csvRowIterator) Next() bool {

	// Check sticky error and returns immediately if not nil.
	if it.decoderError != nil {
		return false
	}

	// This should never occur in normal usage; it indicates that header
	// inference or configuration failed badly.
	if len(it.header) == 0 {
		it.decoderError = errors.New("Header not provided and failed to infer from stream.")
		return false
	}

	// Loop to handle header rows and boundaries between sources.
	for {
		// 1) Serve any pending record from the pushback buffer.
		if it.hasPending {
			it.current = it.pending
			it.currentSrcMeta = it.pendingSrcMeta
			it.hasPending = false
			return true
		}
		row, err := it.csvReader.Read()
		meta := it.srcAwareStream.Current()
		if it.isSourceStart(meta) {
			it.atStart = true
		}
		// At the start of a new source, read and classify exactly one row.
		if it.atStart {
			// read
			if err == io.EOF {
				// Empty source: let the connector advance to the next source.
				it.atStart = false
				it.lastSourceMeta = meta
				continue
			}
			if err != nil {
				it.decoderError = err
				return false
			}
			// 2) if header discrs row and keep looping
			if it.isheader(row) {
				// Source-local header row: drop it and continue the loop
				// to read the first data row of this source
				it.atStart = false
				it.lastSourceMeta = meta
				continue
			}
			// Not a header: store as pending so the normal path can serve it.
			it.pending = row
			it.pendingSrcMeta = meta
			it.hasPending = true
			it.atStart = false
			// serve pending immediately on next iteration
			continue
		}
		// 3) Normal path: read the next data row.
		if err == io.EOF {
			return false
		}
		if err != nil {
			it.decoderError = err
			return false
		}

		it.current = row
		it.currentSrcMeta = meta
		it.lastSourceMeta = meta
		return true
	}
}

// Record returns an Extractor for the current CSV row.
//
// The returned Extractor is only valid until the next call to Next;
// if you need to retain values longer, copy them out.
func (it *csvRowIterator) Record() Extractor {
	return sliceExtractor{current: it.current, header: it.header, invIndex: it.invertedIndex, srcMeta: it.currentSrcMeta}
}

// Err reports the first non-EOF error encountered while decoding.
// Once Err returns a non-nil error, Next will return false.
func (it *csvRowIterator) Err() error {
	return it.decoderError
}

// Close closes the underlying SrcAwareStreamer. It is safe to call Close
// multiple times.
func (it *csvRowIterator) Close() error {
	return it.srcAwareStream.Close()
}

// ByIndex returns the field at i and true if i is within bounds.
func (s sliceExtractor) ByIndex(i int) (string, bool) {
	if i < 0 || i >= len(s.current) {
		return "", false
	}
	return s.current[i], true
}

// ByName returns the field for the given header name and true if present.
func (s sliceExtractor) ByName(name string) (string, bool) {
	idx, ok := s.invIndex[name]
	if !ok {
		return "", false
	}
	return s.current[idx], true
}

// Len reports the number of fields in the current record.
func (s sliceExtractor) Len() int {
	return len(s.current)
}

// Names returns a copy of the header names for this record.
func (s sliceExtractor) Names() []string {
	names := make([]string, 0)
	names = append(names, s.header...)
	return names
}

// Meta returns the source metadata associated with this record.
func (s sliceExtractor) Meta() connector.SrcMeta {
	return s.srcMeta
}

//
// Unexported helpers
//

// isheader reports whether row matches the canonical header exactly.
func (it *csvRowIterator) isheader(row []string) bool {
	hSize, rSize := len(it.header), len(row)
	if hSize != rSize {
		return false
	}
	for i := range hSize {
		if it.header[i] != row[i] {
			return false
		}
	}
	return true
}

// isSourceStart reports whether meta represents the start of a new source
// relative to the last observed SrcMeta.
func (it *csvRowIterator) isSourceStart(meta connector.SrcMeta) bool {
	if it.lastSourceMeta.Name == "" {
		return true
	}
	if meta.Name != it.lastSourceMeta.Name {
		return true
	}
	return meta.ByteOffset == 0 && it.lastSourceMeta.ByteOffset != 0
}

type csvDecoder struct {
	// comma is the rune used as field delimiter during parsing.
	comma rune
	// header holds the canonical header. When empty, it is inferred
	// from the first record in the stream.
	header []string
}

type csvRowIterator struct {
	// csvReader yields one record at a time from the underlying stream.
	csvReader *csv.Reader
	// srcAwareStream exposes both the bytes and their source metadata.
	srcAwareStream connector.SrcAwareStreamer
	// header is the canonical header for all records.
	header []string

	// atStart indicates that we are at the boundary of a new source.
	atStart bool
	// hasPending is true when a peeked record is waiting to be served.
	hasPending bool

	// invertedIndex maps header name → field index in current.
	invertedIndex map[string]int
	// decoderError is a sticky error; once set, Next returns false.
	decoderError error

	// current holds the latest record returned by Next.
	current []string
	// pending holds a single pushed-back record (used when a non-header
	// row is read while atStart == true).
	pending []string

	// currentSrcMeta is the SrcMeta associated with current.
	currentSrcMeta connector.SrcMeta
	// pendingSrcMeta is the SrcMeta associated with pending.
	pendingSrcMeta connector.SrcMeta

	// lastSourceMeta is the last SrcMeta observed from the stream.
	lastSourceMeta connector.SrcMeta
}

// validateHeader checks for basic header sanity (no duplicate names).
func validateHeader(h []string) error {
	names := make(map[string]struct{})
	for _, name := range h {
		if _, ok := names[name]; ok {
			return fmt.Errorf("duplicate entry %s in header %q", name, h)
		}
		names[name] = struct{}{}
	}
	return nil
}

// sliceExtractor is a concrete Extractor backed by a CSV row and header.
type sliceExtractor struct {
	current  []string
	header   []string
	invIndex map[string]int
	srcMeta  connector.SrcMeta
}

// buildIndex constructs a name → index map for the provided header names.
func buildIndex(names []string) map[string]int {
	invIndex := make(map[string]int)
	for idx, name := range names {
		invIndex[name] = idx
	}
	return invIndex
}
