package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// Render returns the full screen, including its header, status footer, and
// final blank line. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.
func (v *View) Render(err error) string {
	added, removed := patchLineCounts(v.projection.patch)
	counts := mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := titleStyle.Render("review-my-slop") + "  " + counts
	body := v.renderBody()
	status := v.renderStatus()
	footer := mutedStyle.Render(status)
	if v.search.repeatMiss {
		message := fmt.Sprintf("no matches for %q", v.search.term)
		footer = errorStyle.Render(message)
	}
	if err != nil {
		footer = errorStyle.Render(strings.ReplaceAll(err.Error(), "\n", " "))
	}
	return frame.Render(header, body, v.renderFooter(footer), v.height)
}

func (v *View) renderBody() []string {
	var body []string
	if len(v.projection.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if v.projection.patch.Kind == patch.Branch {
			empty = "No branch or worktree changes."
		}
		body = make([]string, frame.BodyHeight(v.height))
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(
			v.projection.renderProjection(v.viewport, v.cursor, v.selection),
			"\n",
		)
	}
	return body
}

func (v *View) renderStatus() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if v.search.active {
		status = "/" + v.search.query + cursorStyle.Render(" ")
		if v.search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if v.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return status
}

func (v *View) renderFooter(value string) string {
	label := "local changes"
	if v.projection.patch.Kind == patch.Branch {
		label = "branch changes from " + v.projection.patch.Branch
	}
	if v.viewport.top > 0 {
		label += fmt.Sprintf(
			" (%d%%)",
			v.projection.viewportProgress(v.viewport),
		)
	}
	right := mutedStyle.Render(label)
	width := max(20, v.width)
	value = ansi.Truncate(value, max(0, width-lipgloss.Width(right)-1), "")
	return value + strings.Repeat(
		" ",
		max(1, width-lipgloss.Width(value)-lipgloss.Width(right)),
	) + right
}

func patchLineCounts(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			counts := hunkLineCounts(hunk)
			added += counts[0]
			removed += counts[1]
		}
	}
	return added, removed
}

func hunkLineCounts(hunk patch.Hunk) [2]int {
	counts := [2]int{}
	for _, line := range hunk.Lines {
		switch line.Kind {
		case patch.Addition:
			counts[0]++
		case patch.Deletion:
			counts[1]++
		default:
			continue
		}
	}
	return counts
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
