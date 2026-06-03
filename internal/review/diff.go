package review

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/diffparse"
)

type Runner interface {
	Run(name string, args []string, env []string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args []string, env []string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

type Loader struct {
	Runner Runner
	Parser diffparse.Parser
}

func (l Loader) Load() ([]diffparse.Line, error) {
	runner := l.Runner
	if runner == nil {
		runner = ExecRunner{}
		if _, err := exec.LookPath("difft"); err != nil {
			return nil, errors.New("difftastic is required")
		}
	}
	target, err := l.target(runner)
	if err != nil {
		return nil, err
	}
	out, err := l.renderDiff(runner, target.Args)
	if err != nil {
		return nil, commandError(out, err)
	}
	if target.Local {
		out, err = l.withUntrackedFiles(runner, out)
		if err != nil {
			return nil, err
		}
	}
	parser := l.Parser
	if parser == nil {
		parser = diffparse.Difftastic{}
	}
	return parser.Parse(splitOutput(out)), nil
}

type diffTarget struct {
	Args  []string
	Local bool
}

func (l Loader) target(runner Runner) (diffTarget, error) {
	local, err := hasLocalChanges(runner)
	if err != nil {
		return diffTarget{}, err
	}
	if local {
		args := []string(nil)
		if refExists(runner, "HEAD") {
			args = []string{"HEAD"}
		}
		return diffTarget{Args: args, Local: true}, nil
	}
	if base, ok := branchBase(runner); ok {
		return diffTarget{Args: []string{base + "...HEAD"}}, nil
	}
	if refExists(runner, "HEAD^") {
		return diffTarget{Args: []string{"HEAD^...HEAD"}}, nil
	}
	return diffTarget{Local: true}, nil
}

func (l Loader) renderDiff(runner Runner, args []string) ([]byte, error) {
	gitArgs := append([]string{"-c", "diff.external=difft --color=always", "diff", "--ext-diff", "--color=always"}, args...)
	return runner.Run("git", gitArgs, []string{"DFT_COLOR=always"})
}

func (l Loader) withUntrackedFiles(runner Runner, diff []byte) ([]byte, error) {
	out, err := runner.Run("git", []string{"ls-files", "--others", "--exclude-standard"}, nil)
	if err != nil {
		return nil, commandError(out, err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return diff, nil
	}
	var buf bytes.Buffer
	buf.Write(diff)
	if buf.Len() > 0 && !bytes.HasSuffix(diff, []byte("\n")) {
		buf.WriteByte('\n')
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		rendered, err := runner.Run("difft", []string{"--color=always", "/dev/null", path}, []string{"DFT_COLOR=always"})
		if err != nil || len(bytes.TrimSpace(rendered)) == 0 {
			rendered = plainUntrackedDiff(path)
		}
		if !hasRenderedHeader(rendered) {
			buf.WriteString(path)
			buf.WriteString(" --- Text\n")
		}
		buf.Write(rendered)
		if !bytes.HasSuffix(rendered, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hasRenderedHeader(rendered []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(rendered))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		return strings.Contains(line, " --- ")
	}
	return false
}

func plainUntrackedDiff(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	return renderAddedFile(file)
}

func renderAddedFile(r io.Reader) []byte {
	var buf bytes.Buffer
	scanner := bufio.NewScanner(r)
	line := 1
	for scanner.Scan() {
		fmt.Fprintf(&buf, "\x1b[92;1m %d \x1b[0m%s\n", line, scanner.Text())
		line++
	}
	if line == 1 {
		buf.WriteString("\x1b[92;1m 1 \x1b[0m\n")
	}
	return buf.Bytes()
}

func hasLocalChanges(runner Runner) (bool, error) {
	out, err := runner.Run("git", []string{"status", "--porcelain", "--untracked-files=normal"}, nil)
	if err != nil {
		return false, commandError(out, err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

func branchBase(runner Runner) (string, bool) {
	if ref, ok := gitOutput(runner, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); ok {
		return ref, true
	}
	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		if refExists(runner, ref) {
			return ref, true
		}
	}
	return "", false
}

func refExists(runner Runner, ref string) bool {
	_, err := runner.Run("git", []string{"rev-parse", "--verify", "--quiet", ref + "^{commit}"}, nil)
	return err == nil
}

func gitOutput(runner Runner, args ...string) (string, bool) {
	out, err := runner.Run("git", args, nil)
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(string(out))
	return text, text != ""
}

func commandError(out []byte, err error) error {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return err
	}
	return errors.New(text)
}

func splitOutput(out []byte) []string {
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	out = bytes.TrimRight(out, "\n")
	if len(out) == 0 {
		return []string{""}
	}
	return strings.Split(string(out), "\n")
}
