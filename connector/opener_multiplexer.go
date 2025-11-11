package connector

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/carlodf/cetl/opener"
)

// muxReader multiplexes multiple opener.Opener streams into a single
// io.ReadCloser. It guarantees that only one underlying source is open
// at a time.
//
// Streaming semantics:
//   - Sources are read sequentially in order of the ops slice.
//   - Partial data is preserved on read errors: if a Read(p) returns (n>0, err),
//     the n bytes are forwarded before the error is propagated.
//   - On non-EOF errors, the multiplexer stops streaming and the error is
//     returned to the caller of Read.
//
// Boundary and position tracking:
//   - Current() returns a snapshot of the current source name and byte offset.
//   - AwaitBoundary(ctx) blocks until the next source becomes active, returning
//     its metadata with ByteOffset==0.
//   - The boundary channel is coalesced (buffer=1): if multiple boundaries occur
//     before AwaitBoundary is called, only the latest is delivered.
//
// End-of-stream semantics:
//   - After all sources are exhausted, Read returns io.EOF.
//   - AwaitBoundary returns io.EOF once all boundaries have been consumed and
//     the channel has been drained and closed.
type muxReader struct {
	pr *io.PipeReader
	pw *io.PipeWriter

	// current holds the latest SrcMeta snapshot.
	// Only the multiplexer goroutine writes; readers call Current().
	current atomic.Value

	// boundary communicates source-change events.
	// Buffered size is 1 to coalesce boundaries when the caller
	// is not polling AwaitBoundary().
	boundary chan SrcMeta
}

// Read proxies reads to the underlying io.PipeReader.
// Callers read a continuous byte stream representing all multiplexed sources.
func (m *muxReader) Read(p []byte) (int, error) {
	return m.pr.Read(p)
}

// Close closes the read side of the multiplexer.
// If the internal goroutine has not finished, it will detect the closed pipe
// and terminate early.
func (m *muxReader) Close() error {
	return m.pr.Close()
}

// Current returns the most recent SrcMeta snapshot describing the active
// source and the byte offset within that source.
//
// This is non-blocking and safe to call concurrently with Read and
// AwaitBoundary.
func (m *muxReader) Current() SrcMeta {
	val := m.current.Load()
	if val == nil {
		return SrcMeta{}
	}
	return val.(SrcMeta)
}

// AwaitBoundary waits until the multiplexer switches to a new source,
// returning its metadata (with ByteOffset==0).
//
// If the boundary channel is closed and empty, AwaitBoundary returns io.EOF,
// indicating that there are no more source transitions.
// If ctx is canceled, ctx.Err() is returned.
//
// It is safe to call concurrently with Read.
func (m *muxReader) AwaitBoundary(ctx context.Context) (SrcMeta, error) {
	select {
	case meta, ok := <-m.boundary:
		if !ok {
			return SrcMeta{}, io.EOF
		}
		return meta, nil
	case <-ctx.Done():
		return SrcMeta{}, ctx.Err()
	}
}

// NewMuxReader constructs a SrcAwareStreamer that reads multiple openers
// sequentially and produces a single byte stream.
//
// The provided context controls opening and reading of underlying sources;
// canceling it will abort in-progress reads and shut down the multiplexer.
//
// The returned muxReader implements:
//   - io.ReadCloser via Read and Close
//   - Current() position tracking
//   - AwaitBoundary() source-change notifications
func NewMuxReader(ctx context.Context, ops []opener.Opener) SrcAwareStreamer {
	pr, pw := io.Pipe()
	m := &muxReader{
		pr:       pr,
		pw:       pw,
		boundary: make(chan SrcMeta, 1),
	}

	go func() {
		defer drainAndCloseChannel(m.boundary)
		defer pw.Close()

		buf := make([]byte, 32*1024)
		for _, op := range ops {
			rc, err := op.Open(ctx)
			if err != nil {
				_ = pw.CloseWithError(fmt.Errorf("open %s: %w", op.Name(), err))
				return
			}
			meta := SrcMeta{
				Name:       op.Name(),
				ByteOffset: 0,
			}

			m.current.Store(meta)

			// Communicate new source if client is awaiting for Boundary
			// If no client is awayting and channel is full, the channel
			// is emptied and the latest boudary event is sent.
			overwriteLatest(m.boundary, meta)

			// Stream bytes
			for {
				n, rerr := rc.Read(buf)
				// If n > 0 write on the Pipe before evaluating error as to
				// provide partial bytes in case of read error.
				if n > 0 {
					meta.ByteOffset += int64(n)

					// If writing to Pipe close with error and return.
					if _, werr := m.pw.Write(buf[:n]); werr != nil {
						rc.Close()
						_ = pw.CloseWithError(werr)
						return
					}
					m.current.Store(meta)
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					rc.Close()
					_ = pw.CloseWithError(fmt.Errorf("read %s: %w", op.Name(), rerr))
					return
				}
			}
			rc.Close()
		}
	}()
	return m
}

// overwriteLatest tries to send v on a 1-buffered channel.
// If the buffer is full, it drains one stale value and retries.
// Never blocks indefinitely; guarantees the latest value wins.
func overwriteLatest[T any](ch chan T, v T) {
	for {
		select {
		case ch <- v:
			return
		case <-ch: // drop oldest
			// loop and try sending again
		}
	}
}

func drainAndCloseChannel[T any](ch chan T) {
	for {
		select {
		case <-ch:
		default:
			close(ch)
			return
		}
	}
}
