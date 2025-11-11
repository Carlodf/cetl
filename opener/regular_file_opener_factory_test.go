package opener

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func Test_normalizeFileSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec string
		ok   bool
		want string
	}{
		{"trims whitespace", "   *.psv   ", true, "*.psv"},
		{"unsupported scheme http", "http://example.com/file.txt", false, ""},
		{"malformed file URL parse error", "file://%ZZ", false, ""},
		{"hierarchical file URL with percent decode", "file:///tmp/a%20b.txt", true, filepath.FromSlash("/tmp/a b.txt")},
		{"opaque file URL with percent decode", "file:/tmp/A%20B.txt", true, filepath.FromSlash("/tmp/A B.txt")},
		{"windows style drive trimmed", "file:///C:/Dir/File.txt", true, filepath.FromSlash("C:/Dir/File.txt")},
		{"posix path with colon (not a scheme)", "/tmp/a:b.txt", true, "/tmp/a:b.txt"},
		{"windows drive path (as-is)", `C:\Foo\Bar.txt`, true, `C:\Foo\Bar.txt`},
		{"UNC path (as-is)", `\\server\share\dir\file.txt`, true, `\\server\share\dir\file.txt`},
		{"empty file URI", "file:", false, ""},
		{"file URL with non-local host (UNC-like)", "file://server/share/path/file.txt",
			true, filepath.FromSlash(`//server/share/path/file.txt`)},
		{"short string (len<2) not a drive path", "C", true, "C"},
		// OPAQUE file URL: no "//" → u.Path=="" and u.Opaque!=""
		{"file opaque (windows style)", `file:c:\foo\bar.txt`, true, `c:\foo\bar.txt`},

		// Also opaque: relative path after "file:" (no leading slash)
		{"file opaque (relative)", `file:./rel/thing.txt`, true, filepath.FromSlash("./rel/thing.txt")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeFileSpec(tc.spec)
			if tc.ok {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				if got != tc.want {
					t.Fatalf("want %q, got %q", tc.want, got)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
			}
		})
	}
}

type tf struct {
	Rel  string
	Data string
}

type tcFactory struct {
	name  string
	spec  string
	files []tf
	want  []string // relative to {TMP}, sorted
	err   bool
}

func Test_RegularFileOpenerFactory_Table(t *testing.T) {
	t.Parallel()

	cases := []tcFactory{
		{
			name: "multiple matches sorted",
			spec: "{TMP}/*.txt",
			files: []tf{
				{"c.txt", "c"}, {"a.txt", "a"}, {"b.txt", "b"},
			},
			want: []string{"a.txt", "b.txt", "c.txt"},
		},
		{
			name: "bad glob pattern",
			spec: "{TMP}/[",
			err:  true,
		},
		{
			name:  "no matches",
			spec:  "{TMP}/*.none",
			files: []tf{{"a.txt", ""}},
			err:   true,
		},
		{
			name:  "hierarchical file URL percent decode",
			spec:  "file://{TMP}/a%20b.txt",
			files: []tf{{"a b.txt", "x"}},
			want:  []string{"a b.txt"},
		},
		{
			name:  "opaque file URL (no //) into temp",
			spec:  "file:{TMP}/foo.txt",
			files: []tf{{"foo.txt", "x"}},
			want:  []string{"foo.txt"},
		},
		{
			name: "posix filename containing colon",
			spec: "{TMP}/*.txt",
			files: []tf{
				{"a:b.txt", "ok"},
				{"c.txt", "ok"},
			},
			want: []string{"a:b.txt", "c.txt"},
		},
		{
			name:  "opaque file URL into temp with percent decode",
			spec:  "file:{TMP}/A%20B.txt",
			files: []tf{{"A B.txt", "x"}},
			want:  []string{"A B.txt"},
		},
		{
			name: "hierarchical file URL percent decode in subdir",
			spec: "file://{TMP}/sub/A%20B.txt",
			files: []tf{
				{"sub/A B.txt", "x"},
				{"sub/C.txt", "y"},
			},
			want: []string{"sub/A B.txt"},
		},
		{
			name:  "spec with leading and trailing spaces",
			spec:  "   {TMP}/*.log   ",
			files: []tf{{"x.log", "1"}, {"y.txt", "2"}},
			want:  []string{"x.log"},
		},
		{
			name: "unsupported scheme bubbles up from normalize",
			spec: "http://example.com/file.txt",
			err:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			for _, f := range tc.files {
				full := filepath.Join(root, filepath.FromSlash(f.Rel))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(full, []byte(f.Data), 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			}

			spec := strings.ReplaceAll(tc.spec, "{TMP}", filepath.ToSlash(root))
			ops, err := RegularFileOpenerFactory(spec)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			// Collect using the stable identity:
			got := make([]string, len(ops))
			for i, o := range ops {
				got[i] = o.Name()
			}
			// normalize to relative paths for assertion
			for i := range got {
				rel, err := filepath.Rel(root, got[i])
				if err != nil {
					t.Fatalf("rel: %v", err)
				}
				got[i] = filepath.ToSlash(rel)
			}
			sort.Strings(got)

			want := make([]string, len(tc.want))
			for i, w := range tc.want {
				want[i] = filepath.ToSlash(w)
			}

			if !equalStrings(got, want) {
				t.Fatalf("\nwant: %v\ngot:  %v", want, got)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
