//go:build !windows

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommandStartsAndQuitsInPTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "review-my-slop")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	{
		output, err := build.CombinedOutput()
		testutil.Requiref(t, err == nil, "build command: %v\n%s", err, output)
	}

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "default"},
		{name: "code subcommand", args: []string{"code"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			testCommandStartsAndQuitsInPTY(t, binary, test.args)
		})
	}
}

func testCommandStartsAndQuitsInPTY(t *testing.T, binary string, args []string) {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	testutil.Require(t, err == nil, err)
	runGit(t, repo, "init", "-q")

	testutil.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = repo
	command.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"XDG_DATA_HOME="+t.TempDir(),
		"XDG_STATE_HOME="+t.TempDir(),
	)
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
	testutil.Require(t, err == nil, err)
	defer func() { _ = terminal.Close() }()

	var output lockedBuffer
	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(&output, terminal)
		readDone <- readErr
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), "review-my-slop") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	testutil.Requiref(t, strings.Contains(output.String(), "main.go"), "TUI did not render untracked file:\n%s", output.String())
	{
		_, err := terminal.Write([]byte("q"))
		testutil.Requiref(t, err == nil, "write quit: %v\n%s", err, output.String())
	}
	{
		err := command.Wait()
		testutil.Requiref(t, err == nil, "command exit: %v\n%s", err, output.String())
	}
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("PTY reader did not finish")
	}
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	{
		output, err := command.CombinedOutput()
		testutil.Requiref(t, err == nil, "git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
