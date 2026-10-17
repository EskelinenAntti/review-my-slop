package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func TestWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{20, 80} {
		for _, kind := range []patch.Kind{patch.Unstaged, patch.Branch} {
			p := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			p.Kind, p.Branch = kind, "main"
			m := func() model {
				var (
					p      patch.Patch   = p
					layout initialLayout = initialLayout{size: size{Width: width, Height: 8}}
				)
				inlineLocalSize1 := layout.size
				if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
					inlineLocalSize1 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(p, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			m.err = errors.New(
				"a very long storage failure\n" +
					"that exceeds the available footer width",
			)
			lines := strings.Split(func() string {
				switch m.mode {
				case modeHelp:
					bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
					return helpscreen.Render(bindings, m.width, m.height)
				case modeComments:
					return m.commentView.Render(m.err)
				default:
					return m.diffView.Render(m.err)
				}
			}(), "\n")
			testutil.RequireEqual(t, len(lines), 8,
				"error changed screen height: %d", len(lines),
			)

			footer := lines[len(lines)-2]
			label := "local changes"
			if kind == patch.Branch {
				label = "branch changes from main"
			}
			testutil.RequireEqual(t,
				strings.HasSuffix(ansi.Strip(footer), label), true,
				"footer lost label: %q", footer,
			)
			testutil.RequireEqual(

				// Labels longer than the terminal retain
				// the existing rendering behavior.
				t,
				width < len(label)+1 || lipgloss.Width(footer) == width, true,
				"footer width=%d want=%d", lipgloss.Width(footer), width,
			)
		}
	}
}
