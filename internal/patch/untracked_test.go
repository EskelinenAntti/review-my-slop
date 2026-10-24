package patch

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestLoadUntrackedReadsNULDelimitedPathsInOrder(t *testing.T) {
	root := t.TempDir()
	paths := []string{"space name.txt", "nested/odd\nname.txt", "tab\tname.txt"}
	for _, path := range paths {
		writeFile(t, root, path, path)
	}
	for _, test := range []struct {
		name   string
		output string
		want   []string
	}{
		{name: "empty"},
		{name: "empty entries", output: "\x00\x00"},
		{
			name: "paths with empty entries",
			output: "\x00space name.txt\x00\x00" +
				"nested/odd\nname.txt\x00tab\tname.txt\x00",
			want: paths,
		},
		{
			name:   "no trailing delimiter",
			output: "space name.txt",
			want:   paths[:1],
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := repository{root: root, run: runnerFunc(func(
				_ context.Context, dir string, args ...string,
			) ([]byte, error) {
				if dir != root || !reflect.DeepEqual(args, []string{
					"ls-files", "--others", "--exclude-standard", "-z",
				}) {
					t.Fatalf("unexpected git command in %q: %v", dir, args)
				}
				return []byte(test.output), nil
			})}
			files, err := repo.loadUntracked(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, file := range files {
				got = append(got, file.NewPath)
				if file.NewSource != visibleSource(file.NewPath) {
					t.Errorf("source for %q = %q", file.NewPath, file.NewSource)
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("paths = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoadUntrackedReturnsErrorsWithoutPartialFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "exists.txt", "content\n")
	gitErr := errors.New("git failed")
	for _, test := range []struct {
		name   string
		output string
		runErr error
		cancel bool
		want   error
	}{
		{name: "git error", runErr: gitErr, want: gitErr},
		{
			name: "canceled before reading", output: "missing.txt\x00",
			cancel: true, want: context.Canceled,
		},
		{
			name:   "missing file after valid file",
			output: "exists.txt\x00missing.txt\x00",
			want:   os.ErrNotExist,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := repository{root: root, run: runnerFunc(func(
				context.Context, string, ...string,
			) ([]byte, error) {
				if test.cancel {
					cancel()
				}
				return []byte(test.output), test.runErr
			})}
			files, err := repo.loadUntracked(ctx)
			if files != nil || !errors.Is(err, test.want) {
				t.Fatalf("files = %#v, error = %v; want nil files and %v",
					files, err, test.want,
				)
			}
		})
	}
}
