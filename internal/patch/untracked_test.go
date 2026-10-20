package patch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadUntrackedFilesPreservesPathsAndOrder(t *testing.T) {
	root := t.TempDir()
	paths := []string{"z.txt", "space and\nnewline.txt", "a.txt"}
	for _, path := range paths {
		writeFile(t, root, path, "content\n")
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	files, err := readUntrackedFiles(context.Background(), root,
		[]byte("\x00z.txt\x00\x00directory\x00space and\nnewline.txt\x00a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, file := range files {
		got = append(got, file.NewPath)
	}
	if !reflect.DeepEqual(got, paths) {
		t.Fatalf("paths = %q, want %q", got, paths)
	}
}

func TestReadUntrackedFilesEmptyOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, output := range []string{"", "\x00", "\x00\x00"} {
		files, err := readUntrackedFiles(ctx, t.TempDir(), []byte(output))
		if err != nil || files == nil || len(files) != 0 {
			t.Fatalf("output %q: files = %#v, err = %v", output, files, err)
		}
	}
}

func TestReadUntrackedFilesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, err := readUntrackedFiles(
		ctx, t.TempDir(), []byte("missing.txt\x00"),
	)
	if !errors.Is(err, context.Canceled) || files != nil {
		t.Fatalf("files = %#v, err = %v; want cancellation before file access",
			files, err)
	}
}

func TestReadUntrackedFilesDiscardsPartialResultsOnError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "present.txt", "content\n")
	files, err := readUntrackedFiles(context.Background(), root,
		[]byte("present.txt\x00missing.txt\x00"))
	if !errors.Is(err, os.ErrNotExist) || files != nil {
		t.Fatalf("files = %#v, err = %v; want missing-file error, no results",
			files, err)
	}
}

func TestUntrackedPathsStopsWhenConsumerStops(t *testing.T) {
	var got []string
	for path := range untrackedPaths([]byte("first\x00second\x00")) {
		got = append(got, path)
		break
	}
	if !reflect.DeepEqual(got, []string{"first"}) {
		t.Fatalf("paths = %q, want only first path", got)
	}
}
