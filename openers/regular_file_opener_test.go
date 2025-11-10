package openers

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFileOpener(t *testing.T) {
	testFile := t.TempDir()
	expectedPath := filepath.Join(testFile, "TestFileOpener.md")

	fd, err := os.Create(expectedPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v.", expectedPath)
	}
	testData := []byte("Test data")
	if _, err := fd.Write(testData); err != nil {
		t.Fatalf("Failed to write to test file %s, because of : %v.", expectedPath, err)
	}
	if err := fd.Close(); err != nil {
		t.Fatalf("Failed to close file %s, because of: %v.", expectedPath, err)
	}

	f := NewFile(expectedPath)
	if got := f.Name(); got != expectedPath {
		t.Fatalf("Fail Name() - Expected: %s bit got: %s.", expectedPath, got)
	}
	rc, err := f.Open(context.Background())
	if err != nil {
		t.Fatalf("Fail Open() - because of: %v", err)
	}
	defer rc.Close()

	buff := make([]byte, len(testData))
	read, err := rc.Read(buff)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read from resulting reader: %v.", err)
	}
	if read != len(testData) || string(buff[:read]) != string(testData) {
		t.Fatalf("Expected %v but got %v.", string(testData), string(buff[:read]))
	}
}

func TestFileOpener_CanceledContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "x.psv")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("writefile: %v", err)
	}

	o := NewFile(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Open

	if rc, err := o.Open(ctx); err == nil {
		rc.Close()
		t.Fatalf("Open() with canceled context: got nil error, want ctx.Err()")
	}
}
