package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (m model) requestPatchRefresh() tea.Cmd {
	if m.refreshPatch == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		currentPatch, err := m.refreshPatch(kind)
		return patchRefreshMsg{patch: currentPatch, kind: kind, err: err}
	}
}

func (m *model) applyPatchRefresh(msg patchRefreshMsg) {
	if msg.kind != m.kind {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	m.updatePatch(msg.patch)
	m.err = nil
}

func (m *model) updatePatch(currentPatch patch.Patch) {
	m.currentPatch = currentPatch
	m.patchView.Update(currentPatch)
}
