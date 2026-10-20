package app

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/editor"
)

func (m *model) openCurrentLine() (tea.Cmd, error) {
	path, number, err := m.sourceLocation()
	if err != nil {
		return nil, err
	}
	return editor.OpenSource(m.ctx, path, number, func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	})
}

func (m model) sourceLocation() (string, int, error) {
	file, line, ok := m.diffView.Current()
	if !ok {
		return "", 0, errors.New("select a code line to open in $EDITOR")
	}
	path, number, ok := file.WorkingTreeLocation(m.currentPatch.Root, line)
	if !ok {
		return "", 0, errors.New(
			"current line has no editable working-tree location",
		)
	}
	return path, int(number), nil
}
