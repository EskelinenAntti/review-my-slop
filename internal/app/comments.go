package app

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m *reviewModel) startCommentFromSelection() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	m.edit.body = ""
	m.edit.index = -1
	m.edit.anchor = selectedCommentAnchor(file, lines)
	cmd, err := newCommentEditorCommand(m.ctx, m.edit.body, m.edit.anchor,
		func(body string, err error) tea.Msg {
			return commentEditorFinishedMsg{body: body, err: err}
		})
	if err != nil {
		m.edit = commentEdit{index: -1}
	}
	return cmd, err
}

func selectedCommentAnchor(
	file patches.File, lines []patches.Line,
) feedback.Anchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := feedback.Anchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patches.Addition {
			prefix = "+"
		}
		if line.Kind == patches.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		includeAnchorLine(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		includeAnchorLine(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
}

func includeAnchorLine(start, end *int, number int) {
	if number == 0 {
		return
	}
	if *start == 0 || number < *start {
		*start = number
	}
	if number > *end {
		*end = number
	}
}

func saveReviewComment(
	store commentStore,
) func(feedback.Comment, patches.Patch) (feedback.Comment, error) {
	return func(
		comment feedback.Comment,
		current patches.Patch,
	) (feedback.Comment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return feedback.Comment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}
}
