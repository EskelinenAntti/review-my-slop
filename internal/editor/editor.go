// Package editor provides temporary Markdown files and shell-compatible
// commands for external editors.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Command builds the command used to invoke an external editor. The editor
// string is interpreted by the user's shell, matching the application's
// existing $EDITOR behavior.
func Command(command, path string, line int) *exec.Cmd {
	argument := shellQuote(path)
	if line > 0 {
		argument = "+" + strconv.Itoa(line) + " " + argument
	}
	return exec.Command("sh", "-c", command+" "+argument)
}

// Prepare creates a private Markdown file for an editor session.
func Prepare(command, text string) (*Edit, error) {
	file, err := os.CreateTemp("", "review-my-slop-edit-*.md")
	if err != nil {
		return nil, fmt.Errorf("create editor file: %w", err)
	}
	edit := &Edit{command: command, path: file.Name()}
	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()
		_ = edit.Close()
		return nil, fmt.Errorf("write editor file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = edit.Close()
		return nil, fmt.Errorf("close editor file: %w", err)
	}
	return edit, nil
}

// Edit owns a temporary file for one editor process.
type Edit struct {
	mu      sync.Mutex
	command string
	path    string
	closed  bool
}

// Command returns the external editor command for this file.
func (e *Edit) Command() *exec.Cmd {
	if e == nil {
		return nil
	}
	return Command(e.command, e.path, 0)
}

// Finish reads the edited text and removes the temporary file. A process
// failure also removes the file and is returned to the caller.
func (e *Edit) Finish(processErr error) (string, error) {
	if e == nil {
		return "", fmt.Errorf("editor file is unavailable")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if processErr != nil {
		_ = e.closeLocked()
		return "", fmt.Errorf("editor: %w", processErr)
	}
	data, err := os.ReadFile(e.path)
	closeErr := e.closeLocked()
	if err != nil {
		return "", fmt.Errorf("read editor file: %w", err)
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(data), nil
}

// Close removes the temporary file. It is safe to call more than once.
func (e *Edit) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closeLocked()
}

func (e *Edit) closeLocked() error {
	if e.closed {
		return nil
	}
	e.closed = true
	if err := os.Remove(e.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove editor file: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
