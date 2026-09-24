package ui

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
	review := &m.review
	switch m.mode {
	case modeHelp:
		return m.renderHelp()
	case modeComments:
		return m.renderComments()
	}
	added, removed := patchLineCounts(review.patch)
	header := titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	var body []string
	if len(review.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentBranch() != "" {
			empty = "No branch or worktree changes."
		}
		body = make([]string, m.screenBodyHeight())
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(review.view.Render(review.viewport, review.cursor, review.selection), "\n")
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
	lines := []string{header}
	lines = append(lines, body...)
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func patchLineCounts(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case patch.Addition:
					added++
				case patch.Deletion:
					removed++
				}
			}
		}
	}
	return
}

func (m Model) renderStatus() string {
	search := &m.search
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(search.query) + editorCursorStyle.Render(" ")
		if search.miss {
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
	review := &m.review
	progress := ""
	if review.viewport.Top > 0 {
		progress = fmt.Sprintf(" (%d%%)", review.view.ViewportProgress(review.viewport))
	}
	if branch := m.currentBranch(); branch != "" {
		return "branch changes from " + branch + progress
	}
	return "local changes" + progress
}

func (m Model) renderComments() string {
	state := &m.comments
	header := titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(state.items)))
	height := m.screenBodyHeight()
	width := max(20, m.width)
	body := make([]string, height)
	if len(state.items) == 0 {
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		body = body[:0]
		start := min(max(0, state.row-height+1), max(0, len(state.items)-height))
		end := min(len(state.items), start+height)
		for index := start; index < end; index++ {
			comment := state.items[index]
			prefix, style := "  ", screenContextStyle
			if index == state.row {
				prefix, style = "> ", screenCursorStyle
			}
			anchor := comment.Anchor
			location := anchor.FilePath
			lineNumber := anchor.NewStart
			if lineNumber == 0 {
				lineNumber = anchor.OldStart
			}
			if lineNumber > 0 {
				location += fmt.Sprintf(":%d", lineNumber)
			}
			commentBody := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			line := ansi.Truncate(fmt.Sprintf("%s%s  %s", prefix, location, commentBody), width, "")
			body = append(body, style.Width(width).Render(line))
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{}
	for _, line := range strings.Split(helpText, "\n") {
		keys, description, _ := strings.Cut(line, "\t")
		bindings = append(bindings, keyBinding{keys, description})
	}
	body := append([]string{""}, renderKeyBindings(bindings)...)
	return m.renderScreen(titleStyle.Render("review-my-slop help"), body, mutedStyle.Render("? or Esc closes help"))
}

type keyBinding struct{ keys, description string }

const helpText = `j/k, arrows	move
h/l, left/right	scroll horizontally
Ctrl-w h/l/w	switch side-by-side pane
0/$	start/end of lines
gg/G	first/last changed line
zz/zt/zb	center/top/bottom current line
Ctrl-d/Ctrl-u	half-page down/up
/	search diff text
n/N	next/previous search match
]f/[f	next/previous file
v	select a line range
c	comment on selection/current line
e	open current line in $EDITOR
C	view comments
R	refresh diff
Tab	toggle local/branch changes
t	toggle unified/side-by-side
q	quit`

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	lines := []string{}
	for _, binding := range bindings {
		lines = append(lines, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
	return lines
}

var (
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	screenContextStyle = lipgloss.NewStyle()
	screenCursorStyle  = lipgloss.NewStyle().Reverse(true)
	editorCursorStyle  = lipgloss.NewStyle().Reverse(true)
	mutedStyle         = lipgloss.NewStyle().Faint(true)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
