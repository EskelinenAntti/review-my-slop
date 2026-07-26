package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/repository"
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
	repo, err := repository.New(ctx)
	if err != nil {
		return err
	}
	loadedPatch, err := repo.LocalChanges(ctx)
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
	defaultBranch := repo.DefaultBranch(ctx)
	saveComment := func(comment review.Comment, current repository.Patch) (review.Comment, error) {
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
	model.SetDelete(func(comment review.Comment, current repository.Patch) error {
		return store.Delete(current.Repository, comment.ID)
	})
	model.SetDefaultBranch(defaultBranch)
	model.SetRefresh(func(branch string) (repository.Patch, error) {
		if branch != "" {
			return repo.BranchChanges(ctx, branch)
		}
		return repo.LocalChanges(ctx)
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

func runComments(ctx context.Context, output io.Writer) error {
	repo, err := repository.New(ctx)
	if err != nil {
		return err
	}
	return runCommentsAt(repo, output)
}

func runCommentsAt(repo repository.Repository, output io.Writer) error {
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	comments, err := store.List(repo.Root)
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
	return store.Acknowledge(repo.Root, ids)
}
