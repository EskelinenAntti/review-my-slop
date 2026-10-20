package patch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLimitedFileEnforcesLimitAfterStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", maxFileBytes+1)
	if err := os.WriteFile(path, []byte(large), 0o600); err != nil {
		t.Fatal(err)
	}

	content, oversized, err := readLimitedFile(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if !oversized {
		t.Fatal("oversized file was not reported")
	}
	if len(content) != 0 {
		t.Fatalf("oversized content length = %d, want 0", len(content))
	}
}
