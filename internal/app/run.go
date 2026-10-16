package app

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// RunReviewApp starts an interactive review of the supplied patch.
func RunReviewApp(
	ctx context.Context,
	currentPatch patch.Patch,
	commentStore comments.Store,
	refresh func(context.Context, patch.Kind) (patch.Patch, error),
) error {
	return startApplication(
		ctx,
		currentPatch,
		commentStore,
		refresh,
		readInitialTerminalSize,
		func(
			m model,
			options ...tea.ProgramOption,
		) error {
			_, err := tea.NewProgram(m, options...).Run()
			return err
		},
	)
}

func startApplication(
	ctx context.Context,
	currentPatch patch.Patch,
	store commentStore,
	refresh func(context.Context, patch.Kind) (patch.Patch, error),
	terminalSize func() size,
	launch func(model, ...tea.ProgramOption) error,
) error {
	pending, err := store.ListComments(currentPatch.Root)
	if err != nil {
		return err
	}
	dimensions := terminalSize()
	m, err := newModelWithStore(store, currentPatch, pending, dimensions)
	if err != nil {
		return err
	}
	m.ctx = ctx
	if refresh != nil {
		m.setRefresh(
			func(kind patch.Kind) (patch.Patch, error) {
				return refresh(ctx, kind)
			},
		)
	}
	return launch(
		m,
		tea.WithContext(ctx),
		tea.WithWindowSize(dimensions.Width, dimensions.Height),
	)
}

func readInitialTerminalSize() size {
	return readTerminalSize(term.GetSize)
}

func readTerminalSize(getSize func(uintptr) (int, int, error)) size {
	if width, height, err := getSize(os.Stdin.Fd()); err == nil {
		return size{Width: width, Height: height}
	}
	if width, height, err := getSize(os.Stdout.Fd()); err == nil {
		return size{Width: width, Height: height}
	}
	return defaultSize
}
