// Package ui contains the terminal client for review. It owns layout, cursor
// state, rendering, and the user's external-editor interaction.
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
	files := review.patch.Files
	switch m.mode {
	case modeHelp:
		return m.renderHelp()
	case modeComments:
		return m.renderComments()
	}
	var added, removed int
	for _, file := range files {
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
	header := titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	var body []string
	if len(files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentBranch() != "" {
			empty = "No branch or worktree changes."
		}
		body = make([]string, m.screenBodyHeight())
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(review.view.Render(review.viewport, review.cursor, review.selection), "\n")
	}
	search := &m.search
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(search.query) + editorCursorStyle.Render(" ")
		if search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if review.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	footer := m.renderFooter(mutedStyle.Render(status))
	if m.err != nil {
		footer = m.renderFooter(errorStyle.Render(m.err.Error()))
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderScreen(header string, body []string, footer string) string {
	height := m.screenBodyHeight()
	body = append(body, make([]string, max(0, height-len(body)))...)[:height]
	return strings.Join(append(append([]string{header}, body...), footer, ""), "\n")
}

func (m Model) renderFooter(left string) string {
	review := &m.review
	viewport := review.viewport
	progress := ""
	if viewport.Top > 0 {
		rows := review.view.rows
		progressValue := 0
		if len(rows) > 0 {
			bottom := min(len(rows), viewport.Top+review.view.contentHeight(viewport))
			progressValue = bottom * 100 / len(rows)
		}
		progress = fmt.Sprintf(" (%d%%)", progressValue)
	}
	label := "local changes"
	if branch := m.currentBranch(); branch != "" {
		label = "branch changes from " + branch
	}
	right, width := mutedStyle.Render(label+progress), max(20, m.width)
	rightWidth := lipgloss.Width(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-rightWidth)) + right
}

func (m Model) renderComments() string {
	state := &m.comments
	items := state.items
	header := titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(items)))
	height, width := m.screenBodyHeight(), max(20, m.width)
	body := make([]string, height)
	if len(items) == 0 {
		body[min(1, height-1)] = mutedStyle.Render("No pending comments.")
	} else {
		body = nil
		start := min(max(0, state.row-height+1), max(0, len(items)-height))
		for index, comment := range items[start:min(len(items), start+height)] {
			index += start
			prefix, style := "  ", screenContextStyle
			if index == state.row {
				prefix, style = "> ", screenCursorStyle
			}
			anchor := comment.Anchor
			location, lineNumber := anchor.FilePath, anchor.NewStart
			if lineNumber == 0 {
				lineNumber = anchor.OldStart
			}
			if lineNumber > 0 {
				location += fmt.Sprintf(":%d", lineNumber)
			}
			body = append(body, style.Width(width).Render(
				ansi.Truncate(
					fmt.Sprintf("%s%s  %s", prefix, location, strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")),
					width,
					"",
				),
			))
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderHelp() string {
	bindings, width := []keyBinding{}, 0
	for _, line := range strings.Split(helpText, "\n") {
		keys, description, _ := strings.Cut(line, "\t")
		bindings = append(bindings, keyBinding{keys, description})
		width = max(width, lipgloss.Width(keys))
	}
	body := []string{""}
	for _, binding := range bindings {
		body = append(body, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
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

var (
	baseStyle          = lipgloss.NewStyle()
	titleStyle         = baseStyle.Bold(true).Foreground(lipgloss.Cyan)
	screenContextStyle = baseStyle
	screenCursorStyle  = baseStyle.Reverse(true)
	editorCursorStyle  = baseStyle.Reverse(true)
	mutedStyle         = baseStyle.Faint(true)
	errorStyle         = baseStyle.Foreground(lipgloss.Red).Bold(true)
)
