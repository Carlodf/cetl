package connector

import (
	"context"
	"io"
)

// SrcMeta describes the position of the multiplexer within the current source.
// Name identifies the active source (typically the Opener's Name).
// ByteOffset counts the number of bytes successfully emitted to the reader
// from the current source.
type SrcMeta struct {
	Name       string
	ByteOffset int64
}

type SrcAwareStreamer interface {
	io.ReadCloser

	Current() SrcMeta

	AwaitBoundary(context.Context) (SrcMeta, error)
}
