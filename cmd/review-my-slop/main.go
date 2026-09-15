package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
	"github.com/eskelinenantti/review-my-slop/internal/store"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
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
	loaded, err := (diff.Loader{}).Load(ctx, current)
	if err != nil {
		return err
	}

	inbox, err := store.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := inbox.List(loaded.Repository)
	if err != nil {
		return err
	}
	sideBySide, err := inbox.SideBySide()
	if err != nil {
		return err
	}
	loader := diff.Loader{}
	defaultBranch, err := loader.DefaultBranch(ctx, current)
	if err != nil {
		return err
	}
	saveComment := func(item comment.Comment, current diff.ChangeSet) (comment.Comment, error) {
		item.Repository = current.Repository
		if item.ID != "" {
			return item, inbox.Update(item)
		}
		return inbox.Add(item)
	}
	size := initialTerminalSize()
	model := ui.New(loaded, comments, ui.Dependencies{
		SaveComment: saveComment,
		DeleteComment: func(item comment.Comment, current diff.ChangeSet) error {
			return inbox.Delete(current.Repository, item.ID)
		},
		LoadComments: func() ([]comment.Comment, error) { return inbox.List(loaded.Repository) },
		RefreshDiff: func(branch string) (diff.ChangeSet, error) {
			if branch != "" {
				return loader.LoadBranch(ctx, current, branch)
			}
			return loader.Load(ctx, current)
		},
		SaveSideBySide: inbox.SetSideBySide,
	}, ui.Layout{
		SideBySide: sideBySide,
		Size:       size,
	})
	model.SetDefaultBranch(defaultBranch)
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func initialTerminalSize() ui.Size {
	if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
		return ui.Size{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
		return ui.Size{Width: width, Height: height}
	}
	return ui.DefaultSize
}

func runComments(ctx context.Context, output io.Writer) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCommentsAt(ctx, current, output)
}

func runCommentsAt(ctx context.Context, current string, output io.Writer) error {
	root, err := (diff.Loader{}).Root(ctx, current)
	if err != nil {
		return err
	}
	inbox, err := store.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := inbox.List(root)
	if err != nil {
		return err
	}
	if err := comment.WritePrompt(output, comments); err != nil {
		return err
	}
	ids := make([]string, len(comments))
	for index, item := range comments {
		ids[index] = item.ID
	}
	return inbox.Acknowledge(root, ids)
}
