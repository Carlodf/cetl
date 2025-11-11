package opener

import (
	"fmt"
	"strings"
	"sync"
)

// OpenerFactory constructs an Opener instance from a source specification
// string.
//
// The spec format depends on the opener. For example:
//
//	file opener: "file:///path/to/data.psv" or "/local/path.psv"
//	s3 opener:   "s3://bucket/key.psv"
//
// OpenerFactory is registered by scheme via RegisterOpener.
type OpenerFactory func(spec string) ([]Opener, error)

// RegisterOpener associates a scheme with an OpenerFactory.
//
// This should typically be called from init() within the package that
// implements the opener.
//
// Registration is global for the lifetime of the process. Attempting to
// register the same scheme twice will return an error.
//
// Example:
//
//	func init() {
//	    RegisterOpener(schemeFile, NewFileOpener)
//	}
func RegisterOpener(scheme schemeType, f OpenerFactory) error {
	regMu.Lock()
	defer regMu.Unlock()
	if _, ok := openerRegistry[scheme]; ok {
		return fmt.Errorf("opener for scheme %q already registered", scheme)
	}
	openerRegistry[scheme] = f
	return nil
}

// OpenerFromSpec resolves a source specification string into an Opener
// instance by inferring its scheme.
//
// Behavior:
//
//   - file:// URIs → schemeFile
//   - s3:// URIs   → schemeS3
//   - bare paths   → schemeFile (default fall-through)
//   - unknown schemes return an error
//
// The returned Opener is ready to be used via its Open(ctx) method.
func OpenerFromSpec(spec string) ([]Opener, error) {
	scheme := detectScheme(spec)
	if scheme == schemeUnknown {
		return nil, fmt.Errorf("unknown scheme for %q", spec)
	}
	regMu.RLock()
	f, ok := openerRegistry[scheme]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no opener registered for scheme %q (spec %q)", scheme, spec)
	}
	return f(spec)
}

// schemeType identifies the access mechanism used to retrieve data
// from a source specification.
//
// Examples:
//
//	schemeFile → data is read via local filesystem I/O
//	schemeS3   → data is read via S3 API calls
type schemeType string

const (
	// schemeUnknown indicates that no supported access scheme was detected.
	// OpenerFromSpec will treat this as an error.
	schemeUnknown schemeType = "unknown"
	// schemeFile indicates that data should be accessed via local filesystem
	// operations. This applies to both "file://..." URIs and bare paths.
	schemeFile schemeType = "file"
	// schemeS3 indicates that data should be accessed from Amazon S3.
	// The spec is expected to follow the form "s3://bucket/key".
	schemeS3 schemeType = "s3"
)

var (
	openerRegistry = map[schemeType]OpenerFactory{}
	regMu          sync.RWMutex
)

func detectScheme(spec string) schemeType {
	spec = strings.ToLower(strings.TrimSpace(spec))
	switch {
	case strings.HasPrefix(spec, "file://"):
		return schemeFile
	case strings.HasPrefix(spec, "s3://"):
		return schemeS3
	case !strings.Contains(spec, "://"):
		return schemeFile
	default:
		return schemeUnknown
	}
}
