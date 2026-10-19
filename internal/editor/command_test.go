package editor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommandAdapterStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	adapter := &commandAdapter{}
	adapter.SetStdin(strings.NewReader("input\n"))
	adapter.SetStdout(&stdout)
	adapter.SetStderr(&stderr)
	adapter.run = func() error {
		return adapter.execute(exec.CommandContext(
			context.Background(), "sh", "-c", "cat; printf diagnostic >&2",
		))
	}

	testutil.NoError(t, adapter.Run())
	testutil.CheckEqual(t, stdout.String(), "input\n",
		"stdout=%q", stdout.String(),
	)
	testutil.CheckEqual(t, stderr.String(), "diagnostic",
		"stderr=%q", stderr.String(),
	)
}

func TestCommandAdapterReturnsProcessError(t *testing.T) {
	adapter := &commandAdapter{}
	adapter.run = func() error {
		return adapter.execute(exec.CommandContext(
			context.Background(), "sh", "-c", "exit 7",
		))
	}

	var exitError *exec.ExitError
	err := adapter.Run()
	if !errors.As(err, &exitError) {
		t.Fatalf("expected process exit error, got %v", err)
	}
	testutil.CheckEqual(t, exitError.ExitCode(), 7,
		"exit code=%d", exitError.ExitCode(),
	)
}
