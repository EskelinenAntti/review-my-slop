package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/gitdiff"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

const usage = "usage: review-my-slop [code|comments]"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCode(ctx)
	}
	if len(args) > 1 {
		return errors.New(usage)
	}
	switch args[0] {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; %s", args[0], usage)
	}
}

func runCode(ctx context.Context) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCodeAt(ctx, workingDir, initialTerminalSize)
}

func runCodeAt(ctx context.Context, workingDir string, terminalSize func() tui.Size) error {
	size := terminalSize()
	model, err := newCodeModel(ctx, workingDir, size)
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func newCodeModel(ctx context.Context, workingDir string, size tui.Size) (tui.Model, error) {
	loader := gitdiff.Loader{}
	loadedPatch, err := loader.Load(ctx, workingDir)
	if err != nil {
		return tui.Model{}, err
	}

	store, err := inbox.OpenDefault()
	if err != nil {
		return tui.Model{}, err
	}
	comments, err := store.List(loadedPatch.Repository)
	if err != nil {
		return tui.Model{}, err
	}
	sideBySide, err := store.SideBySideEnabled()
	if err != nil {
		return tui.Model{}, err
	}
	defaultBranch, err := loader.DefaultBranch(ctx, workingDir)
	if err != nil {
		return tui.Model{}, err
	}
	return tui.New(tui.Config{
		Patch:         loadedPatch,
		Comments:      comments,
		Size:          size,
		SideBySide:    sideBySide,
		DefaultBranch: defaultBranch,
		SaveComment: func(comment review.Comment, currentPatch patch.Patch) (review.Comment, error) {
			comment.Repository = currentPatch.Repository
			if comment.ID != "" {
				return comment, store.Update(comment)
			}
			return store.Add(comment)
		},
		DeleteComment: func(comment review.Comment, currentPatch patch.Patch) error {
			return store.Delete(currentPatch.Repository, comment.ID)
		},
		LoadComments: func() ([]review.Comment, error) {
			return store.List(loadedPatch.Repository)
		},
		RefreshDiff: func(branch string) (patch.Patch, error) {
			if branch != "" {
				return loader.LoadBranch(ctx, workingDir, branch)
			}
			return loader.Load(ctx, workingDir)
		},
		SaveSideBySide: store.SetSideBySide,
	}), nil
}

func initialTerminalSize() tui.Size {
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stdout.Fd()} {
		if width, height, err := term.GetSize(fd); err == nil {
			return tui.Size{Width: width, Height: height}
		}
	}
	return tui.DefaultSize
}

func runComments(ctx context.Context, output io.Writer) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCommentsAt(ctx, workingDir, output)
}

func runCommentsAt(ctx context.Context, workingDir string, output io.Writer) error {
	root, err := (gitdiff.Loader{}).Root(ctx, workingDir)
	if err != nil {
		return err
	}
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	return runCommentsWithStore(store, root, output)
}

func runCommentsWithStore(store inbox.Store, root string, output io.Writer) error {
	comments, err := store.List(root)
	if err != nil {
		return err
	}
	if err := inbox.WritePrompt(output, comments); err != nil {
		return err
	}
	return acknowledgeComments(store, root, comments)
}

func acknowledgeComments(store inbox.Store, root string, comments []review.Comment) error {
	ids := make([]string, len(comments))
	for index, comment := range comments {
		ids[index] = comment.ID
	}
	return store.Acknowledge(root, ids)
}
