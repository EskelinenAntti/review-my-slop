package editor

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSourceArgumentsAndContext(t *testing.T) {
	var output bytes.Buffer
	cmd := sourceCommand(
		context.Background(),
		"printf '%s\\n'",
		"/repo with spaces/it's.go",
		42,
	)
	cmd.Stdout = &output
	testutil.NoError(t, cmd.Run())

	if got := output.String(); got != "+42\n/repo with spaces/it's.go\n" {
		t.Fatalf("arguments = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sourceCommand(ctx, "true", "source", 2).Run()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}
