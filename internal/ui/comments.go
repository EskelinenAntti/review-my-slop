package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

// Comments displays pending feedback and owns its selection and scrolling.
type Comments struct {
	items         []feedback.Comment
	row           int
	top           int
	width, height int
}

// Update replaces the items while preserving the selected comment when
// possible.
func (v *Comments) Update(items []feedback.Comment) {
	selected, ok := v.Selected()
	v.items = append([]feedback.Comment(nil), items...)
	v.row = min(v.row, max(0, len(items)-1))
	if ok && selected.ID != "" {
		for index, item := range items {
			if item.ID == selected.ID {
				v.row = index
				break
			}
		}
	}
	v.keepSelectionVisible()
}

// Resize sets the terminal dimensions and keeps the selection visible.
func (v *Comments) Resize(width, height int) {
	v.width, v.height = width, height
	v.keepSelectionVisible()
}

// Move changes the selected row by delta, clamping to the list bounds.
func (v *Comments) Move(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	v.keepSelectionVisible()
}

func (v *Comments) keepSelectionVisible() {
	height := max(1, v.height-3)
	v.top = min(v.top, v.row)
	v.top = max(v.top, v.row-height+1)
	v.top = min(max(0, v.top), max(0, len(v.items)-height))
}

// Selected returns the current comment, or false when the list is empty.
func (v *Comments) Selected() (feedback.Comment, bool) {
	if len(v.items) == 0 {
		return feedback.Comment{}, false
	}
	return v.items[v.row], true
}

// Click selects the item at terminal coordinates and reports whether it exists.
func (v *Comments) Click(x, y int) bool {
	height := (func(height int) int { return max(1, height-3) })(v.height)
	row, ok := (func(x, y, width, height int) (int, bool) {
		row := y - 1
		return row, x >= 0 && x < width && row >= 0 &&
			row < (func(height int) int {
				return max(1, height-3)
			})(height) && y < height-2
	})(x, y, v.width, v.height)
	if !ok {
		return false
	}
	row += min(max(0, v.top), max(0, len(v.items)-height))
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

// Render draws the comment list and an optional error in its footer.
func (v *Comments) Render(err error) string {
	pending := commentscreenMutedStyle.Render(fmt.Sprintf("%d pending", len(
		v.items,
	)))
	header := commentscreenTitleStyle.Render("comments") + "  " + pending
	body := v.renderRows()
	footer := v.renderFooter(err)
	return renderCommentScreenFrame(header, body, footer, v.height)
}

var (
	commentscreenTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Cyan)
	commentscreenMutedStyle = lipgloss.NewStyle().Faint(true)
	commentscreenErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Red).
				Bold(true)
)

func (v *Comments) renderRows() []string {
	height := max(1, v.height-3)
	if len(v.items) == 0 {
		body := make([]string, height)
		row := min(1, height-1)
		body[row] = commentscreenMutedStyle.Render("No pending comments.")
		return body
	}
	start := min(max(0, v.top), max(0, len(v.items)-height))
	end := min(len(v.items), start+height)
	body := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		body = append(body, v.renderRow(index))
	}
	return body
}

func (v *Comments) renderRow(index int) string {
	comment := v.items[index]
	prefix, style := "  ", lipgloss.NewStyle()
	if index == v.row {
		prefix, style = "> ", lipgloss.NewStyle().Reverse(true)
	}
	location := comment.Anchor.FilePath
	if comment.Anchor.NewStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
	} else if comment.Anchor.OldStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
	}
	text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
	line := ansi.Truncate(
		fmt.Sprintf("%s%s  %s", prefix, location, text), max(20, v.width), "",
	)
	return style.Width(max(20, v.width)).Render(line)
}

func (v *Comments) renderFooter(err error) string {
	if err == nil {
		return commentscreenMutedStyle.Render(
			"j/k move  Enter/e edit  D delete  Esc/q return",
		)
	}
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	return commentscreenErrorStyle.Render(ansi.Truncate(message, max(
		20,
		v.width,
	), ""))
}

func renderCommentScreenFrame(
	header string,
	body []string,
	footer string,
	height int,
) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

// NewComments creates a comment list with default terminal dimensions.
func NewComments(items []feedback.Comment) *Comments {
	v := &Comments{width: 80, height: 30}
	v.Update(items)
	return v
}
