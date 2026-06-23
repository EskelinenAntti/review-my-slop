package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(m.render())
	result.AltScreen = true
	result.ReportFocus = true
	return result
}

func (m Model) render() string {
	if m.mode == modeHelp {
		return m.renderHelp()
	}
	if m.mode == modeComments {
		return m.renderComments()
	}
	added, removed := patchLineCounts(m.review.patch)
	header := titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	var body []string
	if len(m.review.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentBranch() != "" {
			empty = "No branch or worktree changes."
		}
		body = make([]string, m.screenBodyHeight())
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(m.review.view.Render(m.review.viewport, m.review.cursor, m.review.selection), "\n")
	}
	footer := m.renderStatus()
	if m.err != nil {
		footer = m.renderFooter(errorStyle.Render(m.err.Error()))
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderScreen(header string, body []string, footer string) string {
	height := m.screenBodyHeight()
	if len(body) > height {
		body = body[:height]
	}
	for len(body) < height {
		body = append(body, "")
	}
	lines := make([]string, 0, height+3)
	lines = append(lines, header)
	lines = append(lines, body...)
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func patchLineCounts(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == patch.Addition {
					added++
				}
				if line.Kind == patch.Deletion {
					removed++
				}
			}
		}
	}
	return
}

func (m Model) renderStatus() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(m.search.query) + editorCursorStyle.Render(" ")
		if m.search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if m.review.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return m.renderFooter(mutedStyle.Render(status))
}

func (m Model) renderFooter(left string) string {
	right := mutedStyle.Render(m.viewLabel())
	width := max(20, m.width)
	rightWidth := lipgloss.Width(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-rightWidth)) + right
}

func (m Model) viewLabel() string {
	progress := ""
	if m.review.viewport.Top.Y > 0 {
		progress = fmt.Sprintf(" (%d%%)", m.review.view.ViewportProgress(m.review.viewport))
	}
	if branch := m.currentBranch(); branch != "" {
		return "branch changes from " + branch + progress
	}
	return "local changes" + progress
}

func (m Model) renderComments() string {
	header := titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(m.comments.items)))
	height := m.screenBodyHeight()
	body := make([]string, 0, height)
	if len(m.comments.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		start := min(max(0, m.comments.row-height+1), max(0, len(m.comments.items)-height))
		end := min(len(m.comments.items), start+height)
		for index := start; index < end; index++ {
			comment := m.comments.items[index]
			prefix, style := "  ", contextStyle
			if index == m.comments.row {
				prefix, style = "> ", cursorStyle
			}
			location := comment.Anchor.FilePath
			if comment.Anchor.NewStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
			} else if comment.Anchor.OldStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
			}
			commentBody := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			line := ansi.Truncate(fmt.Sprintf("%s%s  %s", prefix, location, commentBody), max(20, m.width), "")
			body = append(body, style.Width(max(20, m.width)).Render(line))
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{{"j/k, arrows", "move"}, {"h/l, left/right", "scroll horizontally"}, {"Ctrl-w h/l/w", "switch side-by-side pane"}, {"0/$", "start/end of lines"}, {"gg/G", "first/last changed line"}, {"zz/zt/zb", "center/top/bottom current line"}, {"Ctrl-d/Ctrl-u", "half-page down/up"}, {"/", "search diff text"}, {"n/N", "next/previous search match"}, {"]f/[f", "next/previous file"}, {"v", "select a line range"}, {"c", "comment on selection/current line"}, {"e", "open current line in $EDITOR"}, {"C", "view comments"}, {"R", "refresh diff"}, {"Tab", "toggle local/branch changes"}, {"t", "toggle unified/side-by-side"}, {"q", "quit"}}
	body := append([]string{""}, renderKeyBindings(bindings)...)
	return m.renderScreen(titleStyle.Render("review-my-slop help"), body, mutedStyle.Render("? or Esc closes help"))
}

type keyBinding struct{ keys, description string }

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	lines := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		lines = append(lines, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
	return lines
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	contextStyle      = lipgloss.NewStyle()
	cursorStyle       = lipgloss.NewStyle().Reverse(true)
	editorCursorStyle = lipgloss.NewStyle().Reverse(true)
	mutedStyle        = lipgloss.NewStyle().Faint(true)
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
