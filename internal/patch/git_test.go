package patch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestExecRunnerUsesDirectoryAndReturnsOutput(t *testing.T) {
	repo := newRepository(t)
	directory := filepath.Join(repo, "nested directory")
	testutil.NoError(t, os.Mkdir(directory, 0o700))
	out, err := (execRunner{}).Run(
		context.Background(), directory, "rev-parse", "--show-prefix",
	)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, string(out), "nested directory/\n",
		"output = %q", out,
	)
}

func TestExecRunnerReportsGitStderr(t *testing.T) {
	out, err := (execRunner{}).Run(
		context.Background(), newRepository(t), "not-a-review-command",
	)
	const prefix = "git not-a-review-command: "
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("expected command context and Git stderr, got %v", err)
	}
	stderr := strings.TrimPrefix(err.Error(), prefix)
	if !strings.Contains(stderr, "not-a-review-command") ||
		strings.TrimSpace(stderr) != stderr {
		t.Fatalf("expected trimmed Git stderr, got %q", stderr)
	}
	if out != nil {
		t.Fatalf("failed command returned output %q", out)
	}
}

func TestExecRunnerPreservesStartupError(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "missing")
	out, err := (execRunner{}).Run(context.Background(), directory, "status")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing-directory error, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "git status: ") || out != nil {
		t.Fatalf("output = %q, error = %v", out, err)
	}
}

func TestExecRunnerPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := (execRunner{}).Run(ctx, t.TempDir(), "status")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "git status: ") || out != nil {
		t.Fatalf("output = %q, error = %v", out, err)
	}
}
