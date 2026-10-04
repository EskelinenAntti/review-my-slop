package patch

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/render"
)

func (m *Model) Render() string {
	added, removed := m.patch.Counts()
	header := patchTitleStyle.Render("review-my-slop") + "  " + patchMutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	var body []string
	if len(m.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentBranch() != "" {
			empty = "No branch or worktree changes."
		}
		body = make([]string, m.bodyHeight())
		body[min(1, len(body)-1)] = patchMutedStyle.Render(empty)
	} else {
		body = strings.Split(render.Terminal(m.doc, m.nav.Snapshot(), render.Theme{Dark: m.dark}), "\n")
	}
	height := m.bodyHeight()
	if len(body) > height {
		body = body[:height]
	}
	for len(body) < height {
		body = append(body, "")
	}
	footer := m.renderStatus()
	if m.err != nil {
		footer = m.renderFooter(patchErrorStyle.Render(m.err.Error()))
	}
	lines := make([]string, 0, height+3)
	lines = append(lines, header)
	lines = append(lines, body...)
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func (m *Model) renderStatus() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(m.searchQuery) + patchCursorStyle.Render(" ")
		if m.searchMiss {
			status += patchErrorStyle.Render("  no matches")
		}
	} else if m.nav.Snapshot().Selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return m.renderFooter(patchMutedStyle.Render(status))
}

func (m *Model) renderFooter(left string) string {
	right := patchMutedStyle.Render(m.viewLabel())
	width := max(20, m.width)
	rightWidth := lipgloss.Width(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-rightWidth)) + right
}

func (m *Model) viewLabel() string {
	progress := ""
	snapshot := m.nav.Snapshot()
	if snapshot.Viewport.Top > 0 {
		progress = fmt.Sprintf(" (%d%%)", m.nav.Progress())
	}
	if branch := m.currentBranch(); branch != "" {
		return "branch changes from " + branch + progress
	}
	return "local changes" + progress
}

var (
	patchTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	patchMutedStyle  = lipgloss.NewStyle().Faint(true)
	patchCursorStyle = lipgloss.NewStyle().Reverse(true)
	patchErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
