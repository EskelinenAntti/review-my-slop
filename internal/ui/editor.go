package ui

import (
	"fmt"
	"path/filepath"

	"github.com/eskelinenantti/review-my-slop/internal/comment"

	tea "charm.land/bubbletea/v2"
)

type Editor interface {
	EditComment(body string, anchor comment.Anchor) (tea.Cmd, error)
	OpenSource(path string, line int) (tea.Cmd, error)
}

func (m Model) openCurrentLine() (tea.Cmd, error) {
	file, fileOK := m.layout.view.File(m.review.cursor)
	line, lineOK := m.layout.view.Line(m.review.cursor)
	if !fileOK || !lineOK {
		return nil, fmt.Errorf("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return nil, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.review.changes.Repository, filepath.FromSlash(path))
	}
	if m.dependencies.Editor == nil {
		return nil, fmt.Errorf("editor is unavailable")
	}
	return m.dependencies.Editor.OpenSource(path, int(number))
}
