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
	var out strings.Builder
	added, removed := patchLineCounts(m.patch)
	out.WriteString(titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed)) + "\n")
	if len(m.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentParent() != "" {
			empty = "No branch or worktree changes."
		}
		lines := make([]string, m.viewportHeight())
		lines[min(1, len(lines)-1)] = mutedStyle.Render(empty)
		out.WriteString(strings.Join(lines, "\n"))
		out.WriteByte('\n')
	} else {
		out.WriteString(m.view.Render(m.viewport, m.cursor, m.selection))
		out.WriteByte('\n')
	}
	if m.err != nil {
		out.WriteString(m.renderFooter(errorStyle.Render(m.err.Error())))
	} else {
		out.WriteString(m.renderStatus())
	}
	out.WriteByte('\n')
	return out.String()
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
		status = "/" + string(m.search) + editorCursorStyle.Render(" ")
		if m.searchMiss {
			status += errorStyle.Render("  no matches")
		}
	} else if m.selection != nil {
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
	if m.viewport.Top.Y > 0 {
		progress = fmt.Sprintf(" (%d%%)", m.view.ViewportProgress(m.viewport))
	}
	if parent := m.currentParent(); parent != "" {
		return "branch changes from " + parent + progress
	}
	return "local changes" + progress
}

func (m Model) renderComments() string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(m.comments))) + "\n")
	if len(m.comments) == 0 {
		out.WriteString("\n" + mutedStyle.Render("No pending comments.") + "\n")
	} else {
		for index, comment := range m.comments {
			prefix, style := "  ", contextStyle
			if index == m.commentRow {
				prefix, style = "> ", cursorStyle
			}
			location := comment.Anchor.FilePath
			if comment.Anchor.NewStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
			} else if comment.Anchor.OldStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
			}
			body := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			out.WriteString(style.Width(max(20, m.width)).Render(fmt.Sprintf("%s%s  %s", prefix, location, body)) + "\n")
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	out.WriteString(footer + "\n")
	return out.String()
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{{"j/k, arrows", "move"}, {"h/l, left/right", "scroll horizontally"}, {"Ctrl-w h/l/w", "switch side-by-side pane"}, {"0/$", "start/end of lines"}, {"gg/G", "first/last changed line"}, {"zz/zt/zb", "center/top/bottom current line"}, {"Ctrl-d/Ctrl-u", "half-page down/up"}, {"/", "search diff text"}, {"n/N", "next/previous search match"}, {"]f/[f", "next/previous file"}, {"v", "select a line range"}, {"c", "comment on selection/current line"}, {"e", "open current line in $EDITOR"}, {"C", "view comments"}, {"R", "refresh diff"}, {"Tab", "cycle local/parent branch changes"}, {"t", "toggle unified/side-by-side"}, {"q", "quit"}}
	lines := []string{titleStyle.Render("review-my-slop help"), ""}
	lines = append(lines, renderKeyBindings(bindings)...)
	lines = append(lines, "", mutedStyle.Render("? or Esc closes help"))
	return strings.Join(lines, "\n")
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
