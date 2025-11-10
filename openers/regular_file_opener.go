package openers

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// File is an Opener implementation that provides read access to a regular
// filesystem file. It stores the filesystem path and opens the file lazily.
//
// File does *not* check for existence or file type at construction time.
// This is intentional, to keep opener lightweight and composable.
//
// The identity of the data source is the cleaned file path returned by Name().
type File struct {
	Path string
}

// NewFile constructs a File opener for a given filesystem path. The path is
// cleaned using filepath.Clean, but no existence or permission checks are
// performed. These checks occur when Open is called.
//
// Example:
//
//	o := openers.NewFile("data/input.psv")
//	r, err := o.Open(context.Background())
func NewFile(uri string) File {
	return File{Path: filepath.Clean(uri)}
}

// Open attempts to open the underlying file and returns an io.ReadCloser.
//
// The provided context is checked *before* opening the file. If the context
// is already canceled, Open returns ctx.Err() without performing I/O.
//
// Note: os.Open itself is not context-cancellable, so the context does not
// interrupt the filesystem call once begun. It does, however, provide fast
// short-circuit behavior.
//
// Callers are responsible for closing the returned ReadCloser.
func (f File) Open(ctx context.Context) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return os.Open(f.Path)
}

// Name returns the stable identity of this data source. For File, the identity
// is the cleaned filesystem path. Callers that prefer a basename may use:
//
//	filepath.Base(o.Name())
func (f File) Name() string {
	return f.Path
}
