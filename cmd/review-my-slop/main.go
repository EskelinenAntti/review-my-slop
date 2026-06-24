package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/gitdiff"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	neovimintegration "github.com/eskelinenantti/review-my-slop/internal/neovim"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

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
	switch args[0] {
	case "code":
		if len(args) == 1 {
			return runCode(ctx)
		}
		if len(args) == 2 && args[1] == "--nvim" {
			return runNeovim(ctx, os.Stdin, os.Stdout)
		}
		return usageError()
	case "comments":
		if len(args) != 1 {
			return usageError()
		}
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; %w", args[0], usageError())
	}
}

func usageError() error {
	return errors.New("usage: review-my-slop [code [--nvim]|comments]")
}

func runNeovim(ctx context.Context, input io.Reader, output io.WriteCloser) error {
	return neovimintegration.Serve(ctx, input, output, output)
}

func runCode(ctx context.Context) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	loaded, err := (gitdiff.Loader{}).Load(ctx, current)
	if err != nil {
		return err
	}

	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := store.List(loaded.Repository)
	if err != nil {
		return err
	}
	sideBySide, err := store.SideBySide()
	if err != nil {
		return err
	}
	loader := gitdiff.Loader{}
	defaultBranch, err := loader.DefaultBranch(ctx, current)
	if err != nil {
		return err
	}
	saveComment := func(comment review.Comment, current patch.Patch) (review.Comment, error) {
		return store.Save(comment, current.Repository)
	}
	model := tui.New(loaded, comments, saveComment)
	model.SetLoadComments(func() ([]review.Comment, error) {
		return store.List(loaded.Repository)
	})
	model.SetSideBySide(sideBySide, store.SetSideBySide)
	model.SetDelete(func(comment review.Comment, current patch.Patch) error {
		return store.Delete(current.Repository, comment.ID)
	})
	model.SetDefaultBranch(defaultBranch)
	model.SetRefresh(func(branch string) (patch.Patch, error) {
		if branch != "" {
			return loader.LoadBranch(ctx, current, branch)
		}
		return loader.Load(ctx, current)
	})
	program := tea.NewProgram(model)
	_, err = program.Run()
	return err
}

func runComments(ctx context.Context, output io.Writer) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCommentsAt(ctx, current, output)
}

func runCommentsAt(ctx context.Context, current string, output io.Writer) error {
	root, err := (gitdiff.Loader{}).Root(ctx, current)
	if err != nil {
		return err
	}
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := store.List(root)
	if err != nil {
		return err
	}
	if err := inbox.WritePrompt(output, comments); err != nil {
		return err
	}
	ids := make([]string, len(comments))
	for index, comment := range comments {
		ids[index] = comment.ID
	}
	return store.Acknowledge(root, ids)
}
