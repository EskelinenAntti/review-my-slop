package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/git"
	source "github.com/eskelinenantti/review-my-slop/internal/git"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

var unexpectedArgumentErr = errors.New("usage: review-my-slop [code|comments]")

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	git, err := source.NewGit(ctx)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return runCode(ctx, git)
	}
	if len(args) > 1 {
		return unexpectedArgumentErr
	}

	switch args[0] {
	case "code":
		return runCode(ctx, git)
	case "comments":
		return runComments(git, output)
	default:
		return unexpectedArgumentErr
	}
}

func runCode(ctx context.Context, git git.Git) error {
	git, err := source.NewGit(ctx)
	if err != nil {
		return err
	}
	loadedPatch, err := git.LocalChanges(ctx)
	if err != nil {
		return err
	}
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := store.List(loadedPatch.Repository)
	if err != nil {
		return err
	}
	sideBySide, err := store.SideBySide()
	if err != nil {
		return err
	}
	saveComment := func(comment review.Comment, current source.Patch) (review.Comment, error) {
		comment.Repository = current.Repository
		if comment.ID != "" {
			return comment, store.Update(comment)
		}
		return store.Add(comment)
	}
	size := initialTerminalSize()
	model := tui.New(loadedPatch, comments, saveComment, tui.InitialLayout{
		SideBySide:     sideBySide,
		SaveSideBySide: store.SetSideBySide,
		Size:           size,
	})
	model.SetLoadComments(func() ([]review.Comment, error) {
		return store.List(loadedPatch.Repository)
	})
	model.SetDelete(func(comment review.Comment, current source.Patch) error {
		return store.Delete(current.Repository, comment.ID)
	})
	model.SetDefaultBranch(git.Repository.DefaultBranch)
	model.SetRefresh(func(showBranchChanges bool) (source.Patch, error) {
		if showBranchChanges {
			return git.BranchChanges(ctx)
		}
		return git.LocalChanges(ctx)
	})
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func initialTerminalSize() tui.Size {
	if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
		return tui.Size{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
		return tui.Size{Width: width, Height: height}
	}
	return tui.DefaultSize
}

func runComments(git git.Git, output io.Writer) error {
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := store.List(git.Repository.Root)
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
	return store.Acknowledge(git.Repository.Root, ids)
}
