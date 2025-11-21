package transform

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/carlodf/cetl/connector"
)

func Test_NewDecodeMapTransform_PanicsOnNilDecoder(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil decoder")
		} else if got := fmt.Sprint(r); got != "NewTransform: decoder in nil" {
			t.Fatalf("panic message mismatch: got %q", got)
		}
	}()
	_ = NewDecodeMapTransform[struct{}](nil) // should panic
}

func Test_Transform_NilMapperError_DoesNotCallDecoder(t *testing.T) {
	t.Parallel()
	dec := &stubDecoder{}
	tr := NewDecodeMapTransform[int](dec)

	it, err := tr.Transform(context.Background(), nil, nil)
	if it != nil {
		t.Fatalf("expected nil iterator on error")
	}
	if err == nil || err.Error() != "transform: Mapper[T] must not be nil" {
		t.Fatalf("expected specific error, got: %v", err)
	}
	if dec.called != 0 {
		t.Fatalf("decoder.Decode should not have been called")
	}
}

func Test_Transform_MapsRecordsAndDelegatesClose(t *testing.T) {
	t.Parallel()
	recs := []Extractor{
		stubExtractor{vals: []string{"a", "1"}, names: []string{"colA", "colB"}},
		stubExtractor{vals: []string{"b", "2"}, names: []string{"colA", "colB"}},
	}
	inner := &stubRecordIterator{recs: recs}
	dec := &stubDecoder{recIt: inner}
	type pair struct{ A, B string }
	tr := NewDecodeMapTransform[pair](dec)

	mapCalls := 0
	mapFn := func(e Extractor) (pair, error) {
		mapCalls++
		a, _ := e.ByIndex(0)
		b, _ := e.ByIndex(1)
		return pair{A: a, B: b}, nil
	}

	it, err := tr.Transform(context.Background(), nil, mapFn)
	if err != nil {
		t.Fatalf("unexpected Transform error: %v", err)
	}
	defer it.Close()

	if !it.Next() {
		t.Fatalf("expected first Next to be true")
	}
	if got := it.Struct(); got != (pair{A: "a", B: "1"}) {
		t.Fatalf("first mapped value mismatch: %+v", got)
	}

	if !it.Next() {
		t.Fatalf("expected second Next to be true")
	}
	if got := it.Struct(); got != (pair{A: "b", B: "2"}) {
		t.Fatalf("second mapped value mismatch: %+v", got)
	}

	if it.Next() {
		t.Fatalf("expected EOF (Next == false)")
	}
	if err := it.Err(); err != nil {
		t.Fatalf("expected no error at EOF, got: %v", err)
	}

	if mapCalls != 2 {
		t.Fatalf("expected 2 map calls, got %d", mapCalls)
	}

	if err := it.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !inner.closed {
		t.Fatalf("expected inner iterator to be closed")
	}
}

func Test_MappedIterator_MapErrorStopsAndSurfaces(t *testing.T) {
	t.Parallel()
	recs := []Extractor{stubExtractor{vals: []string{"x"}}}
	inner := &stubRecordIterator{recs: recs}
	dec := &stubDecoder{recIt: inner}
	tr := NewDecodeMapTransform[string](dec)

	wantErr := errors.New("boom")
	mapCalls := 0
	mapFn := func(e Extractor) (string, error) {
		mapCalls++
		return "", wantErr
	}

	it, err := tr.Transform(context.Background(), nil, mapFn)
	if err != nil {
		t.Fatalf("unexpected Transform error: %v", err)
	}
	defer it.Close()

	// First Next should return false due to mapper error.
	if it.Next() {
		t.Fatalf("expected Next == false when mapper errors")
	}
	if got := it.Err(); got == nil || got.Error() != wantErr.Error() {
		t.Fatalf("expected mapper error, got: %v", got)
	}
	if mapCalls != 1 {
		t.Fatalf("expected exactly one map call, got %d", mapCalls)
	}
}

func Test_MappedIterator_InnerErrorPropagates(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("decoder/iterator failure")
	inner := &stubRecordIterator{recs: nil, err: wantErr}
	dec := &stubDecoder{recIt: inner}
	tr := NewDecodeMapTransform[int](dec)

	mapCalls := 0
	mapFn := func(e Extractor) (int, error) {
		mapCalls++
		return 0, nil
	}

	it, err := tr.Transform(context.Background(), nil, mapFn)
	if err != nil {
		t.Fatalf("unexpected Transform error: %v", err)
	}
	defer it.Close()

	if it.Next() {
		t.Fatalf("expected Next == false due to inner error/EOF")
	}
	if got := it.Err(); got == nil || got.Error() != wantErr.Error() {
		t.Fatalf("expected inner error, got: %v", got)
	}
	if mapCalls != 0 {
		t.Fatalf("mapper should not be called when no records; got %d", mapCalls)
	}
}

/**************
   Test stubs
***************/

type stubExtractor struct {
	vals  []string
	names []string
	meta  connector.SrcMeta
}

func (e stubExtractor) ByIndex(i int) (string, bool) {
	if i < 0 || i >= len(e.vals) {
		return "", false
	}
	return e.vals[i], true
}

func (e stubExtractor) ByName(name string) (string, bool) {
	for i, n := range e.names {
		if n == name {
			return e.vals[i], true
		}
	}
	return "", false
}

func (e stubExtractor) Len() int                { return len(e.vals) }
func (e stubExtractor) Names() []string         { return e.names }
func (e stubExtractor) Meta() connector.SrcMeta { return e.meta }

type stubRecordIterator struct {
	recs   []Extractor
	idx    int
	err    error
	closed bool
}

func (s *stubRecordIterator) Next() bool {
	if s.idx >= len(s.recs) {
		return false
	}
	s.idx++
	return true
}

func (s *stubRecordIterator) Record() Extractor { return s.recs[s.idx-1] }
func (s *stubRecordIterator) Err() error        { return s.err }
func (s *stubRecordIterator) Close() error      { s.closed = true; return nil }

type stubDecoder struct {
	recIt  RecordIterator
	err    error
	called int
}

func (d *stubDecoder) Decode(ctx context.Context, rc connector.SrcAwareStreamer) (RecordIterator, error) {
	d.called++
	return d.recIt, d.err
}
