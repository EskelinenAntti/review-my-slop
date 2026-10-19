package editor

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommandAdapterStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	adapter := &commandAdapter{}
	adapter.run = func() error {
		return adapter.execute(exec.Command(
			"sh", "-c", "cat; printf diagnostic >&2",
		))
	}
	adapter.SetStdin(strings.NewReader("input"))
	adapter.SetStdout(&stdout)
	adapter.SetStderr(&stderr)

	testutil.NoError(t, adapter.Run())
	testutil.CheckEqual(t, stdout.String(), "input",
		"stdout=%q", stdout.String(),
	)
	testutil.CheckEqual(t, stderr.String(), "diagnostic",
		"stderr=%q", stderr.String(),
	)
}

func TestCommandAdapterProcessFailure(t *testing.T) {
	adapter := &commandAdapter{}
	adapter.run = func() error {
		return adapter.execute(exec.Command("sh", "-c", "exit 7"))
	}

	var exitError *exec.ExitError
	if err := adapter.Run(); !errors.As(err, &exitError) {
		t.Fatalf("expected process exit error, got %v", err)
	}
	testutil.CheckEqual(t, exitError.ExitCode(), 7,
		"exit code=%d", exitError.ExitCode(),
	)
}
