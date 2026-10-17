package app

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

// Run starts an interactive review of the supplied patch.
func Run(
	ctx context.Context,
	currentPatch patch.Patch,
	inlineLocalCommentStore1 comments.Store,
	refresh func(context.Context, patch.Kind) (patch.Patch, error),
) error {
	return (func(
		ctx context.Context,
		currentPatch patch.Patch,
		store commentStore,
		refresh func(context.Context, patch.Kind) (patch.Patch, error),
		terminalSize func() size,
		launch func(model, ...tea.ProgramOption) error,
	) error {
		pending, err := store.List(currentPatch.Root)
		if err != nil {
			return err
		}
		dimensions := terminalSize()
		m, err := func() (model, error) {
			preferences, err := settings.Load()
			if err != nil {
				return model{}, err
			}
			m := func() model {
				var layout initialLayout = initialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) }, size: dimensions}
				inlineLocalSize1 := layout.size
				if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
					inlineLocalSize1 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: currentPatch, comments: commentState{items: pending}, commentView: commentscreen.New(pending), edit: commentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
					comment.Repository = current.Root
					if comment.ID != "" {
						if err := store.Update(comment); err != nil {
							return comments.Comment{}, err
						}
						return comment, nil
					}
					return store.Add(comment)
				}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(currentPatch, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			m.delete = func(comment comments.Comment, current patch.Patch) error {
				return store.Delete(current.Root, comment.ID)
			}
			m.load = func() ([]comments.Comment, error) { return store.List(currentPatch.Root) }
			return m, nil
		}()
		if err != nil {
			return err
		}
		m.ctx = ctx
		if refresh != nil {
			m.refresh = func(kind patch.Kind) (patch.Patch, error) { return refresh(ctx, kind) }
		}
		return launch(
			m,
			tea.WithContext(ctx),
			tea.WithWindowSize(dimensions.Width, dimensions.Height),
		)
	})(
		ctx,
		currentPatch,
		inlineLocalCommentStore1,
		refresh,
		(func() size {
			if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
				return size{Width: width, Height: height}
			}
			if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
				return size{Width: width, Height: height}
			}
			return defaultSize
		}),
		func(
			m model,
			options ...tea.ProgramOption,
		) error {
			_, err := tea.NewProgram(m, options...).Run()
			return err
		},
	)
}
