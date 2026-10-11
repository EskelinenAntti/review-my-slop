package patch

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(
	t *testing.T,
) {
	repo := branchRepository(t)
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	requireEqual(t, err == nil, true, fixtureV, err)

	requireEqual(
		t,
		got.Root == repo && got.Kind == Branch && got.Branch == fixtureMain,
		true,
		"patch metadata = %#v",
		got,
	)

	want := []string{
		fixtureCommittedTxt,
		fixtureMixedTxt,
		fixtureStagedTxt,
		fixtureUntrackedTxt,
	}
	requireEqual(
		t,
		len(got.Files), len(want),
		"files = %d, want %d: %#v",
		len(got.Files),
		len(want),
		got.Files,
	)
	for i, name := range want {
		requireEqual(
			t,
			got.Files[i].DisplayPath, name,
			"file %d = %q, want %q",
			i,
			got.Files[i].DisplayPath,
			name,
		)
	}
	requireEqual(
		t,
		containsKind(got.Files[0], Addition, "committed on feature") &&
			containsKind(got.Files[1], Addition, "unstaged on feature") &&
			containsKind(got.Files[2], Addition, "staged on feature") &&
			containsKind(got.Files[3], Addition, "untracked on feature"),
		true,
		"branch diff lacks expected changes: %#v",
		got.Files,
	)
	requireEqual(
		t,
		got.Files[0].OldSource == fixtureBaseContent &&
			got.Files[0].NewSource == "committed on feature\n",
		true,
		"unexpected branch sources: old=%q new=%q",
		got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
}

func TestDefaultBranchFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHEAD string
		available  string
		want       string
	}{
		{
			name:       "origin HEAD",
			originHEAD: "origin/trunk",
			want:       "origin/trunk",
		},

		{name: "origin main", available: "origin/main", want: "origin/main"},

		{name: "local main", available: fixtureMain, want: fixtureMain},

		{
			name:      "origin master",
			available: "origin/master",
			want:      "origin/master",
		},

		{name: "local master", available: "master", want: "master"},

		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				root := t.TempDir()
				runner := &defaultBranchRunner{
					root:       root,
					originHEAD: tt.originHEAD,
					available:  tt.available,
				}
				got, err := defaultBranch(context.Background(), runner, root)
				requireEqual(t, err == nil, true, fixtureV, err)
				requireEqual(
					t,
					got, tt.want,
					"default branch = %q, want %q",
					got,
					tt.want,
				)
				for _, call := range runner.calls {
					requireEqual(
						t,
						!strings.Contains(call, "for-each-ref") &&
							!strings.Contains(call, "merge-base") &&
							!strings.Contains(call, "rev-list"),
						true,
						"default branch discovery inspected refs: git %s",
						call,
					)
				}
			},
		)
	}
}

func TestGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", fixtureMain)
	writeFile(t, repo, fixtureDeletedTxt, "delete this\n")
	writeFile(t, repo, fixtureOldGo, "package main\n\nfunc example() {}\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)
	git(t, repo, "switch", "-c", "feature")
	git(t, repo, "mv", fixtureOldGo, fixtureNewGo)
	git(t, repo, "rm", fixtureDeletedTxt)
	writeFile(t, repo, fixtureAddedTxt, "new content\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, "changes")
	writeFile(t, repo, fixtureUntrackedTxt, "untracked content\n")
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	requireEqual(t, err == nil, true, fixtureV, err)
	want := []struct {
		display string
		old     string
		new     string
	}{
		{fixtureAddedTxt, "", fixtureAddedTxt},
		{fixtureDeletedTxt, fixtureDeletedTxt, ""},
		{fixtureNewGo, fixtureOldGo, fixtureNewGo},
		{fixtureUntrackedTxt, "", fixtureUntrackedTxt},
	}
	requireEqual(t, len(got.Files), len(want), "files = %#v", got.Files)
	for i, paths := range want {
		file := got.Files[i]
		checkEqual(
			t,
			file.DisplayPath == paths.display && file.OldPath == paths.old &&
				file.NewPath == paths.new,
			true,
			"file %d paths = %#v, want %#v",
			i,
			file,
			paths,
		)
	}
}

type defaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *defaultBranchRunner) Run(_ context.Context, _ string, args ...string) (

	[]byte,

	error,
) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" &&
		r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case r.matchesAvailableBranch(args):
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}
