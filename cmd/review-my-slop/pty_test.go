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
)

func TestMainCommandStartsAndQuitsInPTY(t *testing.T) {
	binary := buildReviewTestBinary(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "default"},
		{name: "code subcommand", args: []string{"code"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runReviewCommandInPTY(t, binary, test.args...)
		})
	}
}

func buildReviewTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "review-my-slop")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, output)
	}
	return binary
}

func runReviewCommandInPTY(t *testing.T, binary string, args ...string) {
	t.Helper()
	repo := createInitializedGitTestRepository(t)
	writeGitTestFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = repo
	command.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"XDG_DATA_HOME="+t.TempDir(),
		"XDG_STATE_HOME="+t.TempDir(),
	)
	terminal, err := pty.StartWithSize(
		command, &pty.Winsize{Rows: 24, Cols: 100},
	)
	requireNoError(t, err)
	defer terminal.Close()
	var output mainLockedBuffer
	readDone := readPTYOutput(&output, terminal)
	waitForReviewScreen(t, &output)
	quitReviewCommand(t, terminal, command, &output, readDone)
}

func readPTYOutput(output *mainLockedBuffer, terminal io.Reader) <-chan error {
	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(output, terminal)
		readDone <- readErr
	}()
	return readDone
}

func waitForReviewScreen(t *testing.T, output *mainLockedBuffer) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), "review-my-slop") &&
		time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	requireEqual(t, strings.Contains(output.String(), "main.go"), true,
		"TUI did not render untracked file:\n%s", output.String(),
	)
}

func quitReviewCommand(
	t *testing.T,
	terminal *os.File,
	command *exec.Cmd,
	output *mainLockedBuffer,
	readDone <-chan error,
) {
	t.Helper()
	if _, err := terminal.Write([]byte("q")); err != nil {
		t.Fatalf("write quit: %v\n%s", err, output.String())
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("command exit: %v\n%s", err, output.String())
	}
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("PTY reader did not finish")
	}
}

type mainLockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *mainLockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *mainLockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
