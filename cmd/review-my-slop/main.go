package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/gitdiff"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
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
	if len(args) > 1 {
		return fmt.Errorf("usage: review-my-slop [code|comments]")
	}
	switch args[0] {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; usage: review-my-slop [code|comments]", args[0])
	}
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
	parents, err := loader.ParentBranches(ctx, current)
	if err != nil {
		return err
	}
	saveComment := func(comment review.Comment, current patch.Patch) (review.Comment, error) {
		comment.Repository = current.Repository
		if comment.ID != "" {
			return comment, store.Update(comment)
		}
		return store.Add(comment)
	}
	model := tui.New(loaded, comments, saveComment)
	model.SetLoadComments(func() ([]review.Comment, error) {
		return store.List(loaded.Repository)
	})
	model.SetSideBySide(sideBySide, store.SetSideBySide)
	model.SetDelete(func(comment review.Comment, current patch.Patch) error {
		return store.Delete(current.Repository, comment.ID)
	})
	model.SetParents(parents)
	model.SetRefresh(func(parent string) (patch.Patch, error) {
		if parent != "" {
			return loader.LoadBranch(ctx, current, parent)
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
