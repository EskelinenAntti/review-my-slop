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
func Run(ctx context.Context, currentPatch patch.Patch, commentStore comments.Store, refresh func(context.Context, patch.Kind) (patch.Patch, error)) error {
	return run(ctx, currentPatch, commentStore, refresh, initialTerminalSize, func(m model, options ...tea.ProgramOption) error {
		_, err := tea.NewProgram(m, options...).Run()
		return err
	})
}

func run(ctx context.Context, currentPatch patch.Patch, store commentStore, refresh func(context.Context, patch.Kind) (patch.Patch, error), terminalSize func() size, start func(model, ...tea.ProgramOption) error) error {
	pending, err := store.List(currentPatch.Root)
	if err != nil {
		return err
	}
	dimensions := terminalSize()
	m, err := newWithStore(store, currentPatch, pending, dimensions)
	if err != nil {
		return err
	}
	m.ctx = ctx
	if refresh != nil {
		m.setRefresh(func(kind patch.Kind) (patch.Patch, error) { return refresh(ctx, kind) })
	}
	return start(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
}

func initialTerminalSize() size {
	return discoverTerminalSize(term.GetSize)
}

func discoverTerminalSize(getSize func(uintptr) (int, int, error)) size {
	for _, file := range []*os.File{os.Stdin, os.Stdout} {
		if width, height, err := getSize(file.Fd()); err == nil {
			return size{Width: width, Height: height}
		}
	}
	return defaultSize
}
