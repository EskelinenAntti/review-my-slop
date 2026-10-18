package app

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Root = "/tmp/repo with spaces"
	focusLine(t, &m, "new()")
	cmd, err := m.openCurrentLine()
	testutil.RequireEqual(t, err, nil, "command=%v err=%v", cmd, err)
	testutil.RequireEqual(t, cmd != nil, true, "command=%v err=%v", cmd, err)

	path, line, err := m.sourceLocation()
	testutil.RequireEqual(t, err, nil, "location=%q:%d err=%v", path, line, err)
	testutil.RequireEqual(t, path, "/tmp/repo with spaces/main.go",
		"location=%q:%d err=%v", path, line, err,
	)
	testutil.RequireEqual(t, line, 2, "location=%q:%d err=%v", path, line, err)
}

func TestOpenDeletedFileLineRequiresWorkingTreePath(t *testing.T) {
	p := patch.Patch{
		Root: "/repo",
		Files: []patch.File{{
			OldPath:     "removed.go",
			DisplayPath: "removed.go",
			Hunks: []patch.Hunk{{
				Lines: []patch.Line{{
					Kind:      patch.Deletion,
					Text:      "removed()",
					OldNumber: 1,
				}},
			}},
		}},
	}
	m := testModel(p, nil, nil)

	path, line, err := m.sourceLocation()
	testutil.RequireEqual(t, err != nil, true, "location=%q:%d", path, line)
	testutil.RequireEqual(t, path, "", "location=%q:%d", path, line)
	testutil.RequireEqual(t, line, 0, "location=%q:%d", path, line)
	testutil.RequireEqual(
		t,
		err.Error(),
		"current line has no editable working-tree location",
		"location=%q:%d err=%v",
		path,
		line,
		err,
	)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := updateModel(t, testModel(coveragePatch(), nil, nil), textKey("e"))
	testutil.RequireEqual(t, m.err != nil, true, "error = %v", m.err)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}
