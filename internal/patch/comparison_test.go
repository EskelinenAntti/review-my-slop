package patch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestComparison(t *testing.T) {
	mergeBaseErr := errors.New("unrelated histories")
	for _, tc := range []struct {
		name         string
		kind         Kind
		originHEAD   string
		mergeBaseErr error
		cancel       bool
		wantBranch   string
		wantRevision string
		wantError    string
		wantCause    error
		wantCalls    int
	}{
		{
			name: "unstaged skips branch discovery", kind: Unstaged,
		},
		{
			name: "branch uses merge base", kind: Branch,
			originHEAD: " origin/trunk\n", wantBranch: "origin/trunk",
			wantRevision: "base-commit", wantCalls: 2,
		},
		{
			name: "empty default branch", kind: Branch,
			wantError: "default branch is unavailable", wantCalls: 1,
		},
		{
			name: "discovery cancellation", kind: Branch, cancel: true,
			wantError: "discover default branch: context canceled",
			wantCause: context.Canceled, wantCalls: 1,
		},
		{
			name: "merge base failure", kind: Branch,
			originHEAD: "origin/trunk", mergeBaseErr: mergeBaseErr,
			wantError: "find branch point with origin/trunk: " +
				"unrelated histories",
			wantCause: mergeBaseErr, wantCalls: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			calls := 0
			repo := repository{root: root, run: runnerFunc(func(
				gotContext context.Context, dir string, args ...string,
			) ([]byte, error) {
				calls++
				testutil.RequireEqual(t, gotContext, ctx, "Git context changed")
				testutil.RequireEqual(t, dir, root, "Git directory = %q", dir)
				switch command := strings.Join(args, " "); command {
				case "symbolic-ref --quiet --short refs/remotes/origin/HEAD":
					if tc.cancel {
						cancel()
					}
					return []byte(tc.originHEAD), nil
				case "merge-base origin/trunk HEAD":
					return []byte(" base-commit\n"), tc.mergeBaseErr
				default:
					t.Fatalf("unexpected Git command: %s", command)
					return nil, nil
				}
			})}
			branch, revision, err := repo.comparison(ctx, tc.kind)
			testutil.RequireEqual(t, branch, tc.wantBranch,
				"branch = %q, want %q", branch, tc.wantBranch,
			)
			testutil.RequireEqual(t, revision, tc.wantRevision,
				"revision = %q, want %q", revision, tc.wantRevision,
			)
			testutil.RequireEqual(t, calls, tc.wantCalls,
				"Git calls = %d, want %d", calls, tc.wantCalls,
			)
			assertComparisonError(t, err, tc.wantError, tc.wantCause)
		})
	}
}

func assertComparisonError(
	t *testing.T,
	err error,
	message string,
	cause error,
) {
	t.Helper()
	if message == "" {
		testutil.NoError(t, err)
		return
	}
	if err == nil || err.Error() != message {
		t.Fatalf("error = %v, want %q", err, message)
	}
	if cause != nil && !errors.Is(err, cause) {
		t.Fatalf("error = %v, want wrapped %v", err, cause)
	}
}
