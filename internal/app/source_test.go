package app

import (
	"fmt"
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := newTestModel(multiHunkPatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusPatchLine(t, &m, "new()")
	cmd, err := m.openFocusedSourceLine()
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("command=%v err=%v", cmd, err),
	)
	testutil.RequireEqual(t, cmd != nil, true,
		fmt.Sprintf("command=%v err=%v", cmd, err),
	)

	path, line, err := m.sourceEditorTarget()
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("location=%q:%d err=%v", path, line, err),
	)
	testutil.RequireEqual(t, path, "/tmp/repo with spaces/main.go",
		fmt.Sprintf("location=%q:%d err=%v", path, line, err),
	)
	testutil.RequireEqual(t, line, 2,
		fmt.Sprintf("location=%q:%d err=%v", path, line, err),
	)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := applyTestMessage(
		t,
		newTestModel(multiHunkPatch(), nil, nil),
		textKeyMessage("e"),
	)
	testutil.RequireEqual(t, m.err != nil, true,
		fmt.Sprintf("error = %v", m.err),
	)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set",
		fmt.Sprintf("error = %v", m.err),
	)
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	refreshed := multiHunkPatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.setPatchRefresh(
		func(patch.Kind) (patch.Patch, error) { return refreshed, nil },
	)

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = requireAppModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true,
		"editor completion did not refresh",
	)

	m = applyTestMessage(t, m, cmd())
	testutil.RequireEqual(t, slices.Equal(
		m.currentPatch.Files[0].Metadata,
		[]string{"after-editor"},
	), true, fmt.Sprintf("metadata=%q", m.currentPatch.Files[0].Metadata),
	)
}
