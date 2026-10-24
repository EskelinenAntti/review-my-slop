package editor

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommandAdapterForwardsTerminalIO(t *testing.T) {
	var stdout, stderr bytes.Buffer
	adapter := &commandAdapter{}
	adapter.run = func() error {
		return adapter.execute(exec.Command(
			"sh", "-c", `cat; printf diagnostic >&2`,
		))
	}
	adapter.SetStdin(strings.NewReader("input\n"))
	adapter.SetStdout(&stdout)
	adapter.SetStderr(&stderr)

	testutil.NoError(t, adapter.Run())
	testutil.CheckEqual(t, stdout.String(), "input\n",
		"stdout=%q", stdout.String(),
	)
	testutil.CheckEqual(t, stderr.String(), "diagnostic",
		"stderr=%q", stderr.String(),
	)
}

func TestCommandAdapterReturnsWorkflowError(t *testing.T) {
	want := errors.New("workflow failed")
	adapter := &commandAdapter{run: func() error { return want }}
	testutil.CheckEqual(t, adapter.Run(), want,
		"workflow error was not preserved",
	)
}
