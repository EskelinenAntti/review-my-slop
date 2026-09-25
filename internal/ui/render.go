package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m Model) View() tea.View {
	newView := tea.NewView
	if m.quitting {
		return newView("")
	}
	result := newView(m.render())
	result.AltScreen = true
	result.ReportFocus = true
	return result
}

func (m Model) render() string {
	review := m.review
	currentMode := m.mode
	currentPatch := review.patch
	renderMuted := mutedStyle.Render
	switch currentMode {
	case modeHelp:
		return m.renderHelp()
	case modeComments:
		return m.renderComments()
	}
	added, removed := patchLineCounts(currentPatch)
	header := titleStyle.Render("review-my-slop") + "  " + renderMuted(formatString("+%d-%d", added, removed))
	var body []string
	if len(currentPatch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentBranch() != "" {
			empty = "No branch or worktree changes."
		}
		body = make([]string, m.screenBodyHeight())
		body[min(1, len(body)-1)] = renderMuted(empty)
	} else {
		body = splitLines(review.view.Render(review.viewport, review.cursor, review.selection), "\n")
	}
	footer := m.renderStatus()
	if err := m.err; err != nil {
		footer = m.renderFooter(errorStyle.Render(err.Error()))
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
	return joinLines(lines, "\n")
}

func patchLineCounts(p diffPatch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				kind := line.Kind
				switch kind {
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
	review, search := m.review, m.search
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(search.query) + editorCursorStyle.Render(" ")
		if search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if review.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return m.renderFooter(mutedStyle.Render(status))
}

func (m Model) renderFooter(left string) string {
	right := mutedStyle.Render(m.viewLabel())
	width := max(20, m.width)
	rightWidth := widthOf(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	return left + repeat(" ", max(1, width-widthOf(left)-rightWidth)) + right
}

func (m Model) viewLabel() string {
	review := m.review
	viewport := review.viewport
	progress := ""
	if viewport.Top.Y > 0 {
		progress = formatString(" (%d%%)", review.view.ViewportProgress(viewport))
	}
	if branch := m.currentBranch(); branch != "" {
		return "branch changes from " + branch + progress
	}
	return "local changes" + progress
}

func (m Model) renderComments() string {
	renderMuted := mutedStyle.Render
	width := max(20, m.width)
	state := m.comments
	items, row := state.items, state.row
	header := titleStyle.Render("comments") + "  " + renderMuted(formatString("%d pending", len(items)))
	height := m.screenBodyHeight()
	body := make([]string, 0, height)
	if len(items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = renderMuted("No pending comments.")
	} else {
		start := min(max(0, row-height+1), max(0, len(items)-height))
		end := min(len(items), start+height)
		for index := start; index < end; index++ {
			comment := items[index]
			prefix, style := "  ", screenContextStyle
			if index == row {
				prefix, style = "> ", screenCursorStyle
			}
			anchor := comment.Anchor
			location := anchor.FilePath
			newStart, oldStart := anchor.NewStart, anchor.OldStart
			if newStart > 0 {
				location += formatString(":%d", newStart)
			} else if oldStart > 0 {
				location += formatString(":%d", oldStart)
			}
			commentBody := replaceAll(trimSpace(comment.Body), "\n", " ")
			line := ansi.Truncate(formatString("%s%s  %s", prefix, location, commentBody), width, "")
			body = append(body, style.Width(width).Render(line))
		}
	}
	footer := renderMuted("j/k move  Enter/e edit  D delete  Esc/q return")
	if err := m.err; err != nil {
		footer = errorStyle.Render(err.Error())
	}
	return m.renderScreen(header, body, footer)
}

func (m Model) renderHelp() string {
	body := append([]string{""}, renderKeyBindings(helpKeyBindings())...)
	return m.renderScreen(titleStyle.Render("review-my-slop help"), body, mutedStyle.Render("? or Esc closes help"))
}

const helpBindings = `j/k, arrows|move
h/l, left/right|scroll horizontally
Ctrl-w h/l/w|switch side-by-side pane
0/$|start/end of lines
gg/G|first/last changed line
zz/zt/zb|center/top/bottom current line
Ctrl-d/Ctrl-u|half-page down/up
/|search diff text
n/N|next/previous search match
]f/[f|next/previous file
v|select a line range
c|comment on selection/current line
e|open current line in $EDITOR
C|view comments
R|refresh diff
Tab|toggle local/branch changes
t|toggle unified/side-by-side
q|quit`

func helpKeyBindings() []keyBinding {
	var bindings []keyBinding
	for _, line := range splitLines(helpBindings, "\n") {
		keys, description, _ := strings.Cut(line, "|")
		bindings = append(bindings, keyBinding{keys, description})
	}
	return bindings
}

type keyBinding struct{ keys, description string }

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		keys := binding.keys
		width = max(width, widthOf(keys))
	}
	lines := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys := binding.keys
		lines = append(lines, keys+repeat(" ", width-widthOf(keys))+"  "+binding.description)
	}
	return lines
}

var (
	widthOf            = lipgloss.Width
	trimSpace          = strings.TrimSpace
	replaceAll         = strings.ReplaceAll
	repeat             = strings.Repeat
	joinLines          = strings.Join
	splitLines         = strings.Split
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	screenContextStyle = lipgloss.NewStyle()
	screenCursorStyle  = lipgloss.NewStyle().Reverse(true)
	editorCursorStyle  = lipgloss.NewStyle().Reverse(true)
	mutedStyle         = lipgloss.NewStyle().Faint(true)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
