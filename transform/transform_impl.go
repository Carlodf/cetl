package transform

import (
	"context"
	"fmt"

	"github.com/carlodf/cetl/connector"
)

//
// Generic “decoder + mapper” transformer
//

// decodeMapTransform implements Transformer[T] on top of a Decoder and a
// Mapper[T]. It is format-agnostic: you can plug in CSV, XML, JSON…
// as long as they satisfy Decoder.
type decodeMapTransform[T any] struct {
	decoder Decoder
}

// NewDecodeMapTransform constructs a Transformer[T] that uses the provided
// Decoder to turn a SrcAwareStreamer into decoded records, and then applies
// a Mapper[T] to each record to produce strongly-typed values.
//
// The Decoder is typically a CSVDecoder, XMLDecoder, etc.
func NewDecodeMapTransform[T any](decoder Decoder) Transformer[T] {
	if decoder == nil {
		panic("NewTransform: decoder in nil")
	}
	return &decodeMapTransform[T]{decoder: decoder}
}

func (t *decodeMapTransform[T]) Transform(
	ctx context.Context,
	rc connector.SrcAwareStreamer,
	mapFn Mapper[T],
) (StructIterator[T], error) {
	if mapFn == nil {
		return nil, fmt.Errorf("transform: Mapper[T] must not be nil")
	}

	recIt, err := t.decoder.Decode(ctx, rc)
	if err != nil {
		return nil, err
	}

	return &mappedIterator[T]{
		inner: recIt,
		mapFn: mapFn,
	}, nil
}

type mappedIterator[T any] struct {
	inner RecordIterator
	mapFn Mapper[T]

	cur  T
	err  error
	done bool
}

func (m *mappedIterator[T]) Next() bool {
	if m.done {
		return false
	}
	if m.err != nil {
		m.done = true
		return false
	}

	if !m.inner.Next() {
		// EOF or underlying error; caller must inspect Err().
		m.done = true
		return false
	}

	val, err := m.mapFn(m.inner.Record())
	if err != nil {
		m.err = err
		m.done = true
		return false
	}

	m.cur = val
	return true
}

func (m *mappedIterator[T]) Struct() T {
	return m.cur
}

func (m *mappedIterator[T]) Err() error {
	if m.err != nil {
		return m.err
	}
	return m.inner.Err()
}

func (m *mappedIterator[T]) Close() error {
	m.done = true
	return m.inner.Close()
}
