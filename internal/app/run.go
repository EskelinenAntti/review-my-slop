package app

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// Run starts an interactive review of the supplied patch.
func Run(
	ctx context.Context,
	currentPatch patch.Patch,
	commentStore comments.Store,
	refresh func(context.Context, patch.Kind) (patch.Patch, error),
) error {
	return start(
		ctx,
		currentPatch,
		commentStore,
		refresh,
		initialTerminalSize,
		func(
			m model,
			options ...tea.ProgramOption,
		) error {
			_, err := tea.NewProgram(m, options...).Run()
			return err
		},
	)
}

func start(
	ctx context.Context,
	currentPatch patch.Patch,
	store commentStore,
	refresh func(context.Context, patch.Kind) (patch.Patch, error),
	terminalSize func() size,
	launch func(model, ...tea.ProgramOption) error,
) error {
	pendingComments, err := store.List(currentPatch.Root)
	if err != nil {
		return err
	}
	dimensions := terminalSize()
	m, err := newWithStore(store, currentPatch, pendingComments, dimensions)
	if err != nil {
		return err
	}
	m.ctx = ctx
	if refresh != nil {
		m.refresh = func(kind patch.Kind) (patch.Patch, error) {
			return refresh(ctx, kind)
		}
	}
	return launch(
		m,
		tea.WithContext(ctx),
		tea.WithWindowSize(dimensions.width, dimensions.height),
	)
}

func initialTerminalSize() size {
	return discoverTerminalSize(term.GetSize)
}

func discoverTerminalSize(getSize func(uintptr) (int, int, error)) size {
	for _, descriptor := range [...]uintptr{os.Stdin.Fd(), os.Stdout.Fd()} {
		if width, height, err := getSize(descriptor); err == nil {
			return size{width: width, height: height}
		}
	}
	return defaultSize
}
