package openers

import (
	"context"
	"errors"
	"io"
	"maps"
	"strings"
	"sync"
	"testing"
)

// ----- test helpers to snapshot/restore the registry -----

func snapshotRegistry() map[schemeType]OpenerFactory {
	regMu.RLock()
	defer regMu.RUnlock()
	cp := make(map[schemeType]OpenerFactory, len(openerRegistry))
	maps.Copy(cp, openerRegistry)
	return cp
}

func restoreRegistry(saved map[schemeType]OpenerFactory) {
	regMu.Lock()
	defer regMu.Unlock()
	for k := range openerRegistry {
		delete(openerRegistry, k)
	}
	maps.Copy(openerRegistry, saved)
}

// ----- a tiny dummy opener used by tests -----

type dummyOpener struct{ name string }

func (d dummyOpener) Open(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (d dummyOpener) Name() string { return d.name }

// -----------------------------------------------------------

func TestDetectScheme(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want schemeType
	}{
		{"data/a.psv", schemeFile},
		{"./data/a.psv", schemeFile},
		{"  file:///var/a.psv  ", schemeFile},
		{"FILE://C:/tmp/a.psv", schemeFile},
		{"s3://bucket/key", schemeS3},
		{"S3://BUCKET/KEY", schemeS3},
		{"weird://thing", schemeUnknown},
		{"   ", schemeFile}, // bare/empty defaults to file (after TrimSpace -> empty, no "://")
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := detectScheme(tc.in)
			if got != tc.want {
				t.Fatalf("detectScheme(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRegisterOpener_Duplicate(t *testing.T) {

	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// first registration should succeed
	if err := RegisterOpener(schemeFile, func(spec string) ([]Opener, error) {
		return []Opener{dummyOpener{name: spec}}, nil
	}); err != nil {
		t.Fatalf("first RegisterOpener error: %v", err)
	}

	// duplicate should error
	if err := RegisterOpener(schemeFile, func(string) ([]Opener, error) { return nil, nil }); err == nil {
		t.Fatalf("expected duplicate RegisterOpener to error, got nil")
	}
}

func TestOpenerFromSpec_UnknownScheme(t *testing.T) {
	t.Parallel()

	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	_, err := OpenerFromSpec("weird://thing")
	if err == nil {
		t.Fatalf("expected error for unknown scheme")
	}
}

func TestOpenerFromSpec_UsesRegisteredFactory(t *testing.T) {

	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	const wantName = "/tmp/data.psv"
	if err := RegisterOpener(schemeFile, func(spec string) ([]Opener, error) {
		if spec != wantName {
			return nil, errors.New("factory received unexpected spec")
		}
		return []Opener{dummyOpener{name: spec}}, nil
	}); err != nil {
		t.Fatalf("RegisterOpener: %v", err)
	}

	ops, err := OpenerFromSpec(wantName) // bare path → schemeFile
	if err != nil {
		t.Fatalf("OpenerFromSpec: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("OpenerFromSpec should return one opener: %q", ops)
	}
	if got := ops[0].Name(); got != wantName {
		t.Fatalf("opener.Name() = %q, want %q", got, wantName)
	}
}

func TestRegistry_ReadLock_AllowsConcurrentLookups(t *testing.T) {

	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// register once
	if err := RegisterOpener(schemeFile, func(spec string) ([]Opener, error) {
		return []Opener{dummyOpener{name: spec}}, nil
	}); err != nil {
		t.Fatalf("RegisterOpener: %v", err)
	}

	// many concurrent lookups (will also exercise race detector when enabled)
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			spec := "/tmp/x.psv"
			if _, err := OpenerFromSpec(spec); err != nil {
				t.Errorf("lookup %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestOpenerFromSpec_KnownSchemeWithoutFactory(t *testing.T) {
	t.Parallel()

	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Intentionally DO NOT register schemeS3.
	// The scheme is recognized by detectScheme, but the registry lacks a factory.
	_, err := OpenerFromSpec("s3://bucket/key.psv")
	if err == nil {
		t.Fatalf("expected error for known scheme without registered opener (s3), got nil")
	}
	if got := err.Error(); !strings.Contains(got, "no opener registered for scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}
