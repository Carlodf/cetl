package opener

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// RegularFileOpenerFactory returns a slice of Openers that each open a file
// matching the given file specification.
//
// The spec may be one of:
//   - A plain filesystem path or glob (e.g. "/data/*.csv", "logs/*.psv")
//   - A file URL in hierarchical form:  file:///path/to/file.txt
//   - A file URL in opaque form (no "//"): file:/absolute/or/windows/path
//   - A Windows drive path: C:\path\to\file.txt
//   - A Windows UNC path: \\server\share\dir\file.txt
//
// The returned openers are sorted in lexicographical order of their resolved paths.
//
// Examples:
//
//	ops, err := RegularFileOpenerFactory("data/*.csv")
//	ops, err := RegularFileOpenerFactory("file:///tmp/data.csv")
//	ops, err := RegularFileOpenerFactory(`C:\logs\*.txt`)
//
// If no files match, the function returns an error.
// If the spec uses a URL scheme other than "file:", an error is returned.
func RegularFileOpenerFactory(spec string) ([]Opener, error) {
	glob, err := normalizeFileSpec(spec)
	if err != nil {
		return nil, err
	}
	fileNames, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	if len(fileNames) <= 0 {
		return nil, fmt.Errorf("no files matched: %q", glob)
	}
	sort.Strings(fileNames)
	openers := make([]Opener, len(fileNames))
	for i, fName := range fileNames {
		openers[i] = NewFile(fName)
	}
	return openers, nil
}

// normalizeFileSpec converts a user-facing file specification into a form suitable
// for passing to filepath.Glob.
//
// It handles:
//   - Trimming whitespace
//   - Rejecting unsupported URL schemes
//   - Decoding file:// URLs (hierarchical and opaque forms)
//   - Returning Windows drive and UNC paths unchange
func normalizeFileSpec(spec string) (string, error) {
	spec = strings.TrimSpace(spec)

	if scheme, ok := hasSchemeOtherThanFile(spec); ok {
		return "", fmt.Errorf("unsupported scheme %q", scheme)
	}
	if len(spec) >= 5 && strings.EqualFold(spec[:5], "file:") {
		return normalizeFileURL(spec)
	}
	if isWindowsDrivePath(spec) || isUNC(spec) {
		return spec, nil
	}

	return spec, nil
}

// hasSchemeOtherThanFile reports whether the specification begins with a scheme
// other than "file:", while also avoiding falsely treating Windows drive paths
// (e.g. C:\path) as URL schemes.
func hasSchemeOtherThanFile(spec string) (string, bool) {
	if u, err := url.Parse(spec); err == nil && u.Scheme != "" && !strings.EqualFold(u.Scheme, "file") && !isWindowsDrivePath(spec) {
		return u.Scheme, true
	}
	return "", false
}

// normalizeFileURL normalizes a file: URL into a filesystem path.
// Supports:
//   - file:///abs/path
//   - file:/opaque/path
//   - file://server/share/path (UNC)
//
// Percent-encoded sequences are decoded.
// Leading slashes for Windows drive paths (/C:/...) are removed.
func normalizeFileURL(spec string) (string, error) {
	u, err := url.Parse(spec)
	if err != nil {
		return "", err
	}
	path := u.Path
	if u.Path == "" && u.Opaque != "" {
		path = u.Opaque
	} else if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		path = "//" + u.Host + u.Path
	}

	if percentDecode, err := url.PathUnescape(path); err == nil {
		path = percentDecode
	}
	// Strip URL-style leading slash from Windows drive paths: /C:/... → C:/...
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	if path == "" {
		return "", fmt.Errorf("empty file URI: %q", spec)
	}
	return filepath.FromSlash(path), nil
}

// isWindowsDrivePath reports whether spec looks like a Windows drive path
// such as "C:\dir" or "C:/dir".
func isWindowsDrivePath(spec string) bool {
	if len(spec) < 2 {
		return false
	}
	c := spec[0]
	if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
		return false
	}
	if spec[1] != ':' {
		return false
	}
	return len(spec) == 2 || (len(spec) >= 3 && (spec[2] == '\\' || spec[2] == '/'))
}

// isUNC reports whether spec is a Windows UNC path (\\server\share\...).
func isUNC(spec string) bool {
	return strings.HasPrefix(spec, `\\`)
}
