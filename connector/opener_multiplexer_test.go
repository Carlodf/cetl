package connector

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/carlodf/cetl/opener"
)

// ---- fakes ----

type inMemoryReadCloser struct {
	b             []byte
	pos           int
	injectErrorAt int   // if >=0, inject error when pos >= injectErrAt.
	err           error // current error
	closed        bool
}

var injectedError = errors.New("injected read error")

func (rc *inMemoryReadCloser) Read(p []byte) (int, error) {
	if rc.err != nil {
		return 0, rc.err
	}
	if rc.injectErrorAt >= 0 && rc.pos >= rc.injectErrorAt {
		rc.err = injectedError
		return 0, rc.err
	}
	if rc.pos >= len(rc.b) {
		return 0, io.EOF
	}
	n := 0
	if rc.injectErrorAt > 0 {
		n = copy(p, rc.b[rc.pos:rc.injectErrorAt])
	} else {
		n = copy(p, rc.b[rc.pos:])
	}
	rc.pos += n
	return n, nil
}

func (rc *inMemoryReadCloser) Close() error { rc.closed = true; return nil }

type fakeOpener struct {
	name     string
	data     []byte
	openErr  error
	readErrN int // inject error starting at this byte index; <0 => no error
}

func (f fakeOpener) Open(ctx context.Context) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return &inMemoryReadCloser{b: f.data, injectErrorAt: f.readErrN}, nil
}
func (f fakeOpener) Name() string { return f.name }

// ---- tests ----

func TestMuxReader_Concats_Boundaries_Current(t *testing.T) {
	ctx := context.Background()
	ops := []opener.Opener{
		fakeOpener{name: "a", data: []byte("hello"), readErrN: -1},
		fakeOpener{name: "b", data: []byte("WORLD"), readErrN: -1},
	}
	m := NewMuxReader(ctx, ops)

	// Expect two boundary events (one per source), each before any bytes of that source.
	// We pull them before consuming the stream to assert ByteOffset starts at 0.
	meta1, err := m.AwaitBoundary(ctx)
	if err != nil {
		t.Fatalf("AwaitBoundary #1 err: %v", err)
	}
	if meta1.Name != "a" || meta1.ByteOffset != 0 {
		t.Fatalf("meta1 = %+v, want Name=a ByteOffset=0", meta1)
	}

	meta2ch := make(chan SrcMeta, 1)
	err2ch := make(chan error, 1)
	go func() {
		meta2, e := m.AwaitBoundary(ctx)
		meta2ch <- meta2
		err2ch <- e
	}()

	// Read all data
	got, rerr := io.ReadAll(m)
	if rerr != nil {
		t.Fatalf("read all: %v", rerr)
	}
	want := "helloWORLD"
	if string(got) != want {
		t.Fatalf("merged bytes = %q, want %q", got, want)
	}

	// Second boundary should have arrived for "b".
	time.Sleep(time.Second * 1)
	if e := <-err2ch; e != nil && e != io.EOF {
		t.Fatalf("AwaitBoundary #2 sould return ROF. err: %v", e)
	}
	// Current() should reflect the last source with full byte count.
	cur := m.Current()
	if cur.Name != "b" || cur.ByteOffset != int64(len("WORLD")) {
		t.Fatalf("Current() = %+v, want Name=b ByteOffset=%d", cur, len("WORLD"))
	}

	// After stream completion, AwaitBoundary should return io.EOF.
	_, err = m.AwaitBoundary(ctx)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("AwaitBoundary after done err = %v, want io.EOF", err)
	}

	_ = m.Close()
}

func TestMuxReader_ContextCancelOnAwait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ops := []opener.Opener{
		fakeOpener{name: "a", data: []byte("x")},
	}
	m := NewMuxReader(context.Background(), ops)

	// Cancel before awaiting; should return ctx.Err().
	cancel()
	_, err := m.AwaitBoundary(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AwaitBoundary cancel err = %v, want context.Canceled", err)
	}
	_ = m.Close()
}

func TestMuxReader_PropagatesOpenError(t *testing.T) {
	openErr := errors.New("boom")
	ops := []opener.Opener{
		fakeOpener{name: "bad", openErr: openErr},
	}
	m := NewMuxReader(context.Background(), ops)

	// First Read should surface the open error via CloseWithError.
	p := make([]byte, 16)
	_, err := m.Read(p)
	if err == nil || !strings.Contains(err.Error(), "open bad") {
		t.Fatalf("read err = %v, want contains %q", err, "open bad")
	}
	_ = m.Close()
}

func TestMuxReader_PropagatesReadError_WithPartialData(t *testing.T) {
	ops := []opener.Opener{
		fakeOpener{name: "a", data: []byte("abcdef"), readErrN: 3},
	}
	m := NewMuxReader(context.Background(), ops)

	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, m)
	if err == nil || !strings.Contains(err.Error(), "read a") {
		t.Fatalf("io.Copy err = %v, want contains %q", err, "read a")
	}
	// We should have received bytes up to the error point.
	if buf.String() != "abc" {
		t.Fatalf("partial bytes = %q, want %q", buf.String(), "abc")
	}
	_ = m.Close()
}

func TestMuxReader_EmptyOpeners(t *testing.T) {
	m := NewMuxReader(context.Background(), nil)

	// Reading should EOF immediately.
	p := make([]byte, 1)
	n, err := m.Read(p)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read = (%d,%v), want (0,EOF)", n, err)
	}

	// AwaitBoundary should also return EOF right away.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = m.AwaitBoundary(ctx)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("AwaitBoundary err = %v, want io.EOF", err)
	}
	_ = m.Close()
}

func TestMuxReader_BoundaryBuffer_NoDeadlock(t *testing.T) {
	// Ensure non-blocking boundary send does not deadlock when client doesn't call AwaitBoundary immediately.
	ops := []opener.Opener{
		fakeOpener{name: "a", data: []byte("A"), readErrN: -1},
		fakeOpener{name: "b", data: []byte("B"), readErrN: -1},
	}
	ctx := context.Background()
	m := NewMuxReader(ctx, ops)
	defer m.Close()

	// Consume entire stream without calling AwaitBoundary first.
	got, err := io.ReadAll(m)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if string(got) != "AB" {
		t.Fatalf("got %q, want %q", string(got), "AB")
	}

	// Now attempt to drain boundary events: first should be EOF as stream is done.
	time.Sleep(time.Second * 1)
	_, berr := m.AwaitBoundary(ctx)
	if !errors.Is(berr, io.EOF) {
		t.Fatalf("AwaitBoundary after full read err = %v, want io.EOF", berr)
	}
}
