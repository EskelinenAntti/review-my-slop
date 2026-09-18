package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
	"github.com/eskelinenantti/review-my-slop/internal/git"
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
	loader := git.NewLoader(nil)
	loaded, err := loader.Load(ctx, current)
	if err != nil {
		return err
	}

	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	pending, err := store.List(loaded.Repository)
	if err != nil {
		return err
	}
	sideBySide, err := store.SideBySide()
	if err != nil {
		return err
	}
	defaultBranch, err := loader.DefaultBranch(ctx, current)
	if err != nil {
		return err
	}
	saveComment := func(comment comments.Comment, current diff.ChangeSet) (comments.Comment, error) {
		comment.Repository = current.Repository
		if comment.ID != "" {
			return comment, store.Update(comment)
		}
		return store.Add(comment)
	}
	size := initialTerminalSize()
	model := ui.New(loaded, pending, ui.Dependencies{
		SaveComment: saveComment,
		LoadComments: func() ([]comments.Comment, error) {
			return store.List(loaded.Repository)
		},
		DeleteComment: func(comment comments.Comment, current diff.ChangeSet) error {
			return store.Delete(current.Repository, comment.ID)
		},
		RefreshDiff: func(target ui.RefreshTarget) (diff.ChangeSet, error) {
			if target.Mode == ui.BranchChanges {
				return loader.LoadBranch(ctx, current, target.Branch)
			}
			return loader.Load(ctx, current)
		},
		SaveSideBySide: store.SetSideBySide,
	}, ui.Options{
		SideBySide:    sideBySide,
		Size:          size,
		DefaultBranch: defaultBranch,
	})
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
	root, err := git.NewLoader(nil).Root(ctx, current)
	if err != nil {
		return err
	}
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	snapshot, err := store.Snapshot(root)
	if err != nil {
		return err
	}
	if err := comments.WritePrompt(output, snapshot.Comments()); err != nil {
		return err
	}
	return store.Acknowledge(snapshot)
}
