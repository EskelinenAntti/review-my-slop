package review

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Editor interface {
	Open(file string, line int) error
}

type ExecEditor struct{}

func (ExecEditor) Open(file string, line int) error {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vim"
	}
	parts := strings.Fields(editor)
	args := append(parts[1:], fmt.Sprintf("+%d", line), file)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
