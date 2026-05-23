package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

const flowCommand = "flow"

type commandRunner interface {
	Run(name string, args []string, stdin string, stdout, stderr io.Writer) error
	Output(name string, args []string, stdin string) (string, error)
}

type osCommandRunner struct{}

func (osCommandRunner) Run(name string, args []string, stdin string, stdout, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (osCommandRunner) Output(name string, args []string, stdin string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s failed: %s", name, msg)
	}
	return string(out), nil
}

func runFlow(stdout io.Writer) error {
	return runFlowWith(flowConfig{
		Base:     "main",
		Runner:   osCommandRunner{},
		Stdout:   stdout,
		Stderr:   os.Stderr,
		EditFile: openEditorFile,
		Review: func(stdout io.Writer) error {
			return reviewTUI(nil, loadDiffAsync(nil, sourceLocal), stdout)
		},
	})
}

type flowConfig struct {
	Base     string
	Runner   commandRunner
	Stdout   io.Writer
	Stderr   io.Writer
	EditFile func(string) error
	Review   func(io.Writer) error
}

func runFlowWith(cfg flowConfig) error {
	if cfg.Base == "" {
		cfg.Base = "main"
	}
	if cfg.Runner == nil {
		cfg.Runner = osCommandRunner{}
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	if cfg.EditFile == nil {
		cfg.EditFile = openEditorFile
	}
	if cfg.Review == nil {
		cfg.Review = func(stdout io.Writer) error {
			return reviewTUI(nil, loadDiffAsync(nil, sourceLocal), stdout)
		}
	}

	if err := requireTools(cfg.Runner, "git", "gh", "codex"); err != nil {
		return err
	}
	if err := requireCleanWorktree(cfg.Runner); err != nil {
		return err
	}

	description, err := editPrompt(cfg.EditFile, "Describe the change Codex should make. This becomes the draft PR description.\n")
	if err != nil {
		return err
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return errors.New("empty prompt; flow cancelled")
	}

	title := prTitle(description)
	branch := branchName(title)
	fmt.Fprintf(cfg.Stdout, "Creating branch %s from %s.\n", branch, cfg.Base)
	if err := cfg.Runner.Run("git", []string{"switch", cfg.Base}, "", cfg.Stdout, cfg.Stderr); err != nil {
		return err
	}
	if err := cfg.Runner.Run("git", []string{"pull", "--ff-only", "origin", cfg.Base}, "", cfg.Stdout, cfg.Stderr); err != nil {
		return err
	}
	if err := cfg.Runner.Run("git", []string{"switch", "-c", branch}, "", cfg.Stdout, cfg.Stderr); err != nil {
		return err
	}

	if err := runCodex(cfg, description); err != nil {
		return err
	}
	if err := commitAll(cfg.Runner, cfg.Stdout, cfg.Stderr, "Codex implementation"); err != nil {
		return err
	}
	if err := cfg.Runner.Run("git", []string{"push", "-u", "origin", branch}, "", cfg.Stdout, cfg.Stderr); err != nil {
		return err
	}

	prURL, err := createDraftPR(cfg, title, description, cfg.Base)
	if err != nil {
		return err
	}
	fmt.Fprintf(cfg.Stdout, "Draft PR created: %s\n", strings.TrimSpace(prURL))

	for {
		if err := cfg.Review(cfg.Stdout); err != nil {
			return err
		}
		pr := github.DetectPR()
		if pr == nil {
			return errors.New("no active GitHub PR found after review")
		}
		threads, err := github.UnresolvedThreads(pr)
		if err != nil {
			return err
		}
		if len(threads) == 0 {
			next, err := editPrompt(cfg.EditFile, "No unresolved review comments were found.\n\nAdd another Codex prompt, or leave empty / write READY to mark the PR ready for review.\n")
			if err != nil {
				return err
			}
			next = strings.TrimSpace(next)
			if next == "" || strings.EqualFold(next, "ready") || strings.EqualFold(next, "no more changes") {
				return markReady(cfg, pr)
			}
			if err := runCodex(cfg, next); err != nil {
				return err
			}
			if err := commitAll(cfg.Runner, cfg.Stdout, cfg.Stderr, "Codex follow-up"); err != nil {
				return err
			}
			if err := cfg.Runner.Run("git", []string{"push"}, "", cfg.Stdout, cfg.Stderr); err != nil {
				return err
			}
			continue
		}

		prompt := reviewFixPrompt(threads)
		if err := runCodex(cfg, prompt); err != nil {
			return err
		}
		if err := commitAll(cfg.Runner, cfg.Stdout, cfg.Stderr, "Address review comments"); err != nil {
			return err
		}
		if err := cfg.Runner.Run("git", []string{"push"}, "", cfg.Stdout, cfg.Stderr); err != nil {
			return err
		}
		for _, thread := range threads {
			if err := github.ResolveThread(thread.ID); err != nil {
				return err
			}
		}
		fmt.Fprintf(cfg.Stdout, "Resolved %d review %s.\n", len(threads), plural(len(threads), "thread", "threads"))
	}
}

func requireTools(runner commandRunner, names ...string) error {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

func requireCleanWorktree(runner commandRunner) error {
	out, err := runner.Output("git", []string{"status", "--porcelain"}, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return errors.New("worktree has uncommitted changes; commit or stash them before starting the flow")
	}
	return nil
}

func editPrompt(editFile func(string) error, header string) (string, error) {
	file, err := os.CreateTemp("", "rms-prompt-*.md")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)

	template := "<!-- " + strings.TrimSpace(header) + " -->\n\n"
	if _, err := file.WriteString(template); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := editFile(path); err != nil {
		return "", err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return stripHTMLComments(string(body)), nil
}

func stripHTMLComments(text string) string {
	re := regexp.MustCompile(`(?s)<!--.*?-->`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}

func runCodex(cfg flowConfig, prompt string) error {
	fmt.Fprintln(cfg.Stdout, "Running codex exec.")
	return cfg.Runner.Run("codex", []string{"exec", "--cd", mustGetwd(), "--sandbox", "workspace-write", "-"}, prompt, cfg.Stdout, cfg.Stderr)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func commitAll(runner commandRunner, stdout, stderr io.Writer, message string) error {
	status, err := runner.Output("git", []string{"status", "--porcelain"}, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return errors.New("codex produced no changes to commit")
	}
	if err := runner.Run("git", []string{"add", "-A"}, "", stdout, stderr); err != nil {
		return err
	}
	return runner.Run("git", []string{"commit", "-m", message}, "", stdout, stderr)
}

func createDraftPR(cfg flowConfig, title, body, base string) (string, error) {
	file, err := os.CreateTemp("", "rms-pr-body-*.md")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := os.WriteFile(path, []byte(body+"\n"), 0600); err != nil {
		return "", err
	}
	return cfg.Runner.Output("gh", []string{"pr", "create", "--draft", "--base", base, "--title", title, "--body-file", path}, "")
}

func markReady(cfg flowConfig, pr *github.PR) error {
	if pr == nil {
		return errors.New("no active GitHub PR found")
	}
	fmt.Fprintln(cfg.Stdout, "Marking PR ready for review.")
	return cfg.Runner.Run("gh", []string{"pr", "ready", fmt.Sprintf("%d", pr.Number)}, "", cfg.Stdout, cfg.Stderr)
}

func reviewFixPrompt(threads []github.Thread) string {
	var b strings.Builder
	b.WriteString("Address the following GitHub pull request review comments. Make the requested changes, run appropriate checks, and leave the working tree ready to commit.\n\n")
	for i, thread := range threads {
		fmt.Fprintf(&b, "%d. %s", i+1, thread.Path)
		if thread.Line > 0 {
			fmt.Fprintf(&b, ":%d", thread.Line)
		}
		b.WriteString("\n")
		for _, comment := range thread.Comments {
			author := strings.TrimSpace(comment.Author)
			if author == "" {
				author = "reviewer"
			}
			fmt.Fprintf(&b, "   %s: %s\n", author, strings.TrimSpace(comment.Body))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func prTitle(description string) string {
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			return line
		}
	}
	return "Codex change"
}

func branchName(title string) string {
	title = strings.ToLower(title)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := strings.Trim(re.ReplaceAllString(title, "-"), "-")
	if slug == "" {
		slug = "change"
	}
	return slug
}
