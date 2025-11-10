package openers

import (
	"context"
	"io"
)

type Opener interface {
	Open(ctx context.Context) (io.ReadCloser, error)
	Name() string
}
