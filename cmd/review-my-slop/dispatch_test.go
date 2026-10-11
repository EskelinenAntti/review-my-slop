package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExitCodePrintsOnlyFailures(t *testing.T) {
	var stderr bytes.Buffer
	if got := exitCode(nil, &stderr); got != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", got, stderr.String())
	}
	if got := exitCode(errors.New("failed"), &stderr); got != 1 || stderr.String() != "review-my-slop: failed\n" {
		t.Fatalf("code=%d stderr=%q", got, stderr.String())
	}
}

func TestCodeCommandsPropagateCanceledLoad(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, args := range [][]string{nil, {"code"}} {
		if err := run(ctx, args, &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
	if err := run(ctx, []string{"code", "extra"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("err=%v", err)
	}
}

func TestCommentsSubcommandDispatch(t *testing.T) {
	t.Chdir(initRepository(t))
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var output bytes.Buffer
	if err := run(context.Background(), []string{"comments"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "No pending review comments.\n" {
		t.Fatalf("output=%q", output.String())
	}
}
