package git

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenBindsRepositoryRootAndLoadsFromSubdirectory(t *testing.T) {
	root := newRepository(t)
	writeFile(t, root, "nested/file.go", "package nested\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "base")
	writeFile(t, root, "nested/file.go", "package nested\nfunc Changed() {}\n")

	repository, err := Open(context.Background(), filepath.Join(root, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if repository.Root() != root {
		t.Fatalf("root = %q, want %q", repository.Root(), root)
	}
	got, err := repository.Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != root || len(got.Files) != 1 || got.Files[0].DisplayPath != "nested/file.go" {
		t.Fatalf("loaded patch = %#v", got)
	}
}
