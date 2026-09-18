package ui

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
)

func Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: review-my-slop [code|comments]")
	}
	if len(args) == 0 || args[0] == "code" {
		return runCode(ctx)
	}
	if args[0] == "comments" {
		return runComments(ctx, output)
	}
	return fmt.Errorf("unknown subcommand %q; usage: review-my-slop [code|comments]", args[0])
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
	size := InitialSize(os.Stdin, os.Stdout)
	model := New(loaded, pending, Dependencies{
		SaveComment: saveComment,
		LoadComments: func() ([]comments.Comment, error) {
			return store.List(loaded.Repository)
		},
		DeleteComment: func(comment comments.Comment, current diff.ChangeSet) error {
			return store.Delete(current.Repository, comment.ID)
		},
		RefreshDiff: func(target RefreshTarget) (diff.ChangeSet, error) {
			if target.Mode == BranchChanges {
				return loader.LoadBranch(ctx, current, target.Branch)
			}
			return loader.Load(ctx, current)
		},
		SaveSideBySide: store.SetSideBySide,
	}, Options{
		SideBySide:    sideBySide,
		Size:          size,
		DefaultBranch: defaultBranch,
	})
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func InitialSize(stdin, stdout *os.File) Size {
	if width, height, err := term.GetSize(stdin.Fd()); err == nil {
		return Size{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(stdout.Fd()); err == nil {
		return Size{Width: width, Height: height}
	}
	return DefaultSize
}

func runComments(ctx context.Context, output io.Writer) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := git.NewLoader(nil).Root(ctx, current)
	if err != nil {
		return err
	}
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	return store.Deliver(root, output)
}
