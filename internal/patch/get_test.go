package patch

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := newRepository(t)
	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 1 }\n",
	)
	writeFile(t, repo, fixtureStagedTxt, "before\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)

	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 2 }\n",
	)
	writeFile(t, repo, fixtureStagedTxt, "after\n")
	git(t, repo, gitAddCommand, fixtureStagedTxt)
	writeFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)

	assertUnstagedFileList(t, got, repo)
	modified := got.Files[0]
	requireEqual(
		t,
		containsKind(modified, Deletion, "return 1") &&
			containsKind(modified, Addition, "return 2"),
		true,
		"modified file lacks expected lines: %#v",
		modified.Hunks,
	)
	untracked := got.Files[1]
	requireEqual(
		t,
		untracked.OldSource == "" &&
			strings.Contains(untracked.NewSource, "def hello"),
		true,
		"unexpected untracked sources: %#v",
		untracked,
	)
	assertUntrackedLines(t, untracked)
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := addedFile("odd\nname.go", "package main\n")
	requireEqual(
		t,
		file.NewPath, "odd\nname.go",
		"raw path = %q",
		file.NewPath,
	)
	requireEqual(
		t,
		file.DisplayPath, `odd\nname.go`,
		"display path = %q",
		file.DisplayPath,
	)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		got.Root == repo && len(got.Files) == 1, true,
		"patch = %#v",
		got,
	)
	file := got.Files[0]
	requireEqual(
		t,
		file.OldPath == "" && file.NewPath == "nested/odd\nname.go" &&
			file.DisplayPath == `nested/odd\nname.go`,
		true,
		"file paths = %#v",
		file,
	)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(newRepository(t))
	local, err := Get(context.Background(), Unstaged)
	requireEqual(
		t,
		err == nil && local.Branch == "", true,
		"local patch = %#v, error = %v",
		local,
		err,
	)
	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(unknownPatchKind), "unknown patch kind"},
	} {
		got, err := Get(context.Background(), test.kind)
		requireEqual(
			t,
			err != nil && strings.Contains(err.Error(), test.message) &&
				reflect.DeepEqual(got, Patch{}),
			true,
			"kind %v: patch = %#v, error = %v",
			test.kind,
			got,
			err,
		)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := Get(context.Background(), Unstaged)
	requireEqual(
		t,
		err != nil && reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v",
		got,
		err,
	)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(newRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Get(ctx, Unstaged)
	requireEqual(
		t,
		errors.Is(err, context.Canceled) && reflect.DeepEqual(got, Patch{}),
		true,
		"patch = %#v, error = %v",
		got,
		err,
	)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+1))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(t, len(got.Files), 1, "files = %#v", got.Files)
	file := got.Files[0]
	requireEqual(
		t,
		file.OldPath == "" && file.NewPath == "large.txt" &&
			file.NewSource == "" &&
			len(file.Hunks) == 0 &&
			strings.Contains(
				strings.Join(file.Metadata, "\n"),
				"exceeds 2 MiB",
			),
		true,
		"large file = %#v",
		file,
	)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, fixtureTrackedBin, "\x00old")
	git(t, repo, gitAddCommand, fixtureTrackedBin)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)
	writeFile(t, repo, fixtureTrackedBin, "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(got.Files), 2,
		"files = %d, want 2: %#v",
		len(got.Files),
		got.Files,
	)
	for _, file := range got.Files {
		checkEqual(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != fixtureTrackedBin ||
					file.OldPath == fixtureTrackedBin),
			true,
			"binary paths = %#v",
			file,
		)
		requireEqual(
			t,
			len(file.Hunks), 0,
			"binary file %q has diff lines: %#v",
			file.DisplayPath,
			file.Hunks,
		)
		requireEqual(
			t,
			strings.Contains(
				strings.ToLower(strings.Join(file.Metadata, "\n")),
				"binary",
			),
			true,
			"binary file %q lacks metadata: %#v",
			file.DisplayPath,
			file.Metadata,
		)
	}
}
