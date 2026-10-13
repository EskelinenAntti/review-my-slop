package patch

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDefaultBranchFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHEAD string
		available  string
		want       string
	}{
		{name: "origin HEAD", originHEAD: "origin/trunk", want: "origin/trunk"},
		{name: "origin main", available: "origin/main", want: "origin/main"},
		{name: "local main", available: "main", want: "main"},
		{
			name:      "origin master",
			available: "origin/master",
			want:      "origin/master",
		},
		{name: "local master", available: "master", want: "master"},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			runner := &defaultBranchRunner{
				root:       root,
				originHEAD: tt.originHEAD,
				available:  tt.available,
			}
			got, err := defaultBranch(context.Background(), runner, root)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, got, tt.want,
				fmt.Sprintf("default branch = %q, want %q", got, tt.want),
			)

			for _, call := range runner.calls {
				testutil.RequireEqual(t,
					!strings.Contains(call, "for-each-ref"), true,
					fmt.Sprintf(
						"default branch discovery inspected refs: git %s",
						call,
					),
				)
				testutil.RequireEqual(t, !strings.Contains(call, "merge-base"),
					true, fmt.Sprintf(
						"default branch discovery inspected refs: git %s",
						call,
					),
				)
				testutil.RequireEqual(t, !strings.Contains(call, "rev-list"),
					true, fmt.Sprintf(
						"default branch discovery inspected refs: git %s",
						call,
					),
				)
			}
		})
	}
}

type defaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *defaultBranchRunner) Run(
	_ context.Context,
	_ string,
	args ...string,
) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" &&
		r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case call == "rev-parse --verify --quiet "+r.available+"^{commit}" &&
		r.available != "":
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}
