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

func TestCommandStartsAndQuitsInPTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "review-my-slop")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	{
		output, err := build.CombinedOutput()
		requireEqual(
			t,
			err == nil, true,
			"build command: %v\n%s",
			err,
			output,
		)
	}

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "default"},
		{name: "code subcommand", args: []string{"code"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertPTYStartup(t, binary, test.args)
		})
	}
}

func assertPTYStartup(t *testing.T, binary string, args []string) {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	requireEqual(t, err == nil, true, fixtureV, err)
	runGit(t, repo, "init", "-q")
	{
		err := os.WriteFile(
			filepath.Join(repo, "main.go"),
			[]byte("package main\n\nfunc main() {}\n"),
			publicFileMode,
		)
		requireEqual(t, err == nil, true, fixtureV, err)
	}

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
		command,
		&pty.Winsize{Rows: 24, Cols: 100},
	)
	requireEqual(t, err == nil, true, fixtureV, err)
	defer terminal.Close()

	var output lockedBuffer
	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(&output, terminal)
		readDone <- readErr
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), "review-my-slop") &&
		time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	requireEqual(
		t,
		strings.Contains(output.String(), "main.go"), true,
		"TUI did not render untracked file:\n%s",
		output.String(),
	)
	{
		_, err := terminal.Write([]byte("q"))
		requireEqual(
			t,
			err == nil, true,
			"write quit: %v\n%s",
			err,
			output.String(),
		)
	}
	{
		err := command.Wait()
		requireEqual(
			t,
			err == nil, true,
			"command exit: %v\n%s",
			err,
			output.String(),
		)
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
		requireEqual(
			t,
			err == nil, true,
			"git %s: %v\n%s",
			strings.Join(args, " "),
			err,
			output,
		)
	}
}
