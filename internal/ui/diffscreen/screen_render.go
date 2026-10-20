package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// Render returns the full screen, including its header, status footer, and
// final blank line. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.
func (v *View) Render(err error) string {
	counts := frame.Muted(v.view.countsLabel)
	header := frame.Title(appinfo.Name) + "  " + counts
	body := v.renderBody()
	status := v.renderStatus()
	footer := frame.Muted(status)
	if v.search.repeatMiss {
		footer = v.search.renderRepeatMiss()
	}
	if err != nil {
		footer = frame.Error(frame.SingleLine(err.Error()))
	}
	return frame.Render(header, body, v.renderFooter(footer), v.height)
}

func (v *View) renderBody() []string {
	if len(v.view.patch.Files) > 0 {
		return v.view.render(v.viewport, v.cursor, v.selection)
	}
	if v.view.patch.Kind == patch.Branch {
		return frame.EmptyBody(v.height, "No branch or worktree changes.")
	}
	return frame.EmptyBody(v.height, "No unstaged or untracked changes.")
}

func (v *View) renderStatus() string {
	if v.search.active {
		return v.search.renderQuery()
	}
	if v.selection != nil {
		return "visual selection  j/k extend  c comment  Esc cancel"
	}
	return "j/k/h/l move  c comment  ? help  q quit"
}

func (v *View) renderFooter(value string) string {
	label := "local changes"
	if v.view.patch.Kind == patch.Branch {
		label = "branch changes from " + v.view.patch.Branch
	}
	if v.viewport.top > 0 {
		label += fmt.Sprintf(" (%d%%)", v.view.viewportProgress(v.viewport))
	}
	right := frame.Muted(label)
	width := frame.ScreenWidth(v.width)
	rightWidth := lipgloss.Width(right)
	value = ansi.Truncate(value, max(0, width-rightWidth-1), "")
	return value + strings.Repeat(
		" ",
		max(1, width-lipgloss.Width(value)-rightWidth),
	) + right
}

func (s searchState) renderQuery() string {
	status := "/" + s.query + cursorStyle.Render(" ")
	if s.miss {
		status += frame.Error("  no matches")
	}
	return status
}

func (s searchState) renderRepeatMiss() string {
	return frame.Error(fmt.Sprintf("no matches for %q", s.term))
}
