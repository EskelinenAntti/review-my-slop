package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

// Run executes the CLI command and writes pending feedback to output.
func Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCodeReviewCommand(ctx)
	}
	if len(args) > 1 {
		return errors.New("usage: review-my-slop [code|comments]")
	}
	switch args[0] {
	case "code":
		return runCodeReviewCommand(ctx)
	case "comments":
		return writeCommandPendingComments(ctx, output)
	default:
		return fmt.Errorf(
			"unknown subcommand %q; usage: review-my-slop [code|comments]",
			args[0],
		)
	}
}

func writeCommandPendingComments(ctx context.Context, output io.Writer) error {
	commentStore, err := userCommentsStore()
	if err != nil {
		return err
	}
	currentPatch, err := patches.Load(ctx, patches.Unstaged)
	if err != nil {
		return err
	}
	return commentStore.WritePending(output, currentPatch.Root)
}

func runCodeReviewCommand(ctx context.Context) error {
	commentStore, err := userCommentsStore()
	if err != nil {
		return err
	}
	currentPatch, err := patches.Load(ctx, patches.Unstaged)
	if err != nil {
		return err
	}
	return runReviewApplication(
		ctx,
		currentPatch,
		commentStore,
		patches.Load,
		currentTerminalSize,
		launchReviewProgram,
	)
}

func currentTerminalSize() size {
	if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
		return size{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
		return size{Width: width, Height: height}
	}
	return defaultSize
}

func launchReviewProgram(m reviewModel, options ...tea.ProgramOption) error {
	_, err := tea.NewProgram(m, options...).Run()
	return err
}

func runReviewApplication(
	ctx context.Context,
	currentPatch patches.Patch,
	store commentStore,
	refresh func(context.Context, patches.Kind) (patches.Patch, error),
	terminalSize func() size,
	launch func(reviewModel, ...tea.ProgramOption) error,
) error {
	pending, err := store.List(currentPatch.Root)
	if err != nil {
		return err
	}
	dimensions := terminalSize()
	m, err := buildReviewModel(currentPatch, dimensions, pending, store)
	if err != nil {
		return err
	}
	m.ctx = ctx
	if refresh != nil {
		m.refresh = func(kind patches.Kind) (patches.Patch, error) {
			return refresh(ctx, kind)
		}
	}
	return launch(
		m,
		tea.WithContext(ctx),
		tea.WithWindowSize(dimensions.Width, dimensions.Height),
	)
}

func buildReviewModel(
	currentPatch patches.Patch,
	dimensions size,
	pending []feedback.Comment,
	store commentStore,
) (reviewModel, error) {
	path, err := userSettingsPath()
	if err != nil {
		return reviewModel{}, err
	}
	preferences, err := readSettingsPreferences(path)
	if err != nil {
		return reviewModel{}, err
	}
	m := createReviewAppModel(
		currentPatch,
		dimensions,
		pending,
		preferences,
		store,
	)
	m.delete = func(comment feedback.Comment, current patches.Patch) error {
		return store.Delete(current.Root, comment.ID)
	}
	m.load = func() ([]feedback.Comment, error) {
		return store.List(currentPatch.Root)
	}
	return m, nil
}
