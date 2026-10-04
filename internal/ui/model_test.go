package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func testModel(p patch.Patch, comments []comments.Comment, save saveCommentFunc) model {
	return newModel(p, comments, save, initialLayout{size: size{Width: 100, Height: 30}})
}

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newWithStore(store, modelPatch(), nil, defaultSize)
	if err != nil {
		t.Fatal(err)
	}
	anchor := comments.Anchor{FilePath: "main.go", NewStart: 2, QuotedLines: []string{"+new()"}}
	m.comments.body = "check this"
	m.comments.editAnchor = anchor
	m.finishCommentEdit()
	if m.err != nil {
		t.Fatal(m.err)
	}
	loaded, err := m.load()
	if err != nil || len(loaded) != 1 || loaded[0].Repository != "/repo" || loaded[0].ID == "" || !slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines) {
		t.Fatalf("loaded = %#v, error = %v", loaded, err)
	}
	id := loaded[0].ID
	m.comments.editIndex = 0
	m.comments.body = "edited"
	m.finishCommentEdit()
	if m.err != nil {
		t.Fatal(m.err)
	}
	loaded, err = m.load()
	if err != nil || len(loaded) != 1 || loaded[0].ID != id || loaded[0].Body != "edited" {
		t.Fatalf("edited = %#v, error = %v", loaded, err)
	}
	m.deleteComment(0)
	if m.err != nil {
		t.Fatal(m.err)
	}
	loaded, err = m.load()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("remaining = %#v, error = %v", loaded, err)
	}
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 120, Height: 30},
	})
	if !m.review.sideBySide || !m.sideBySideActive() || !strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.review.sideBySide, m.sideBySideActive(), m.render())
	}
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 80, Height: 30},
	})
	if !m.review.sideBySide || m.sideBySideActive() || strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.review.sideBySide, m.sideBySideActive(), m.render())
	}

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	if !m.review.sideBySide || !m.sideBySideActive() || !strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.review.sideBySide, m.sideBySideActive(), m.render())
	}
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newModel(modelPatch(), nil, nil, initialLayout{
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		size: size{Width: 120, Height: 30},
	})

	m = updateModel(t, m, textKey("t"))
	m = updateModel(t, m, textKey("t"))
	if !slices.Equal(saved, []bool{true, false}) {
		t.Fatalf("saved=%v", saved)
	}
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.move(1)
	diffSelection := m.review.view.beginSelection(m.review.cursor)
	m.review.selection = &diffSelection
	m.move(1)
	want, _ := m.review.view.line(m.review.cursor)
	refreshed := modelPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.rebuildView(refreshed)
	got, ok := m.review.view.line(m.review.cursor)
	if !ok || got != want {
		t.Fatalf("diffCursor line = %#v, want %#v", got, want)
	}
	if m.review.selection == nil || len(m.review.view.lines(*m.review.selection)) != 2 {
		t.Fatalf("diffSelection was not translated: %#v", m.review.selection)
	}
}

func TestUnchangedRefreshRebuildsAndPreservesState(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newModel(p, nil, nil, initialLayout{size: size{Width: 40, Height: 8}})
	for range 8 {
		m.move(forward)
	}
	diffSelection := m.review.view.beginSelection(m.review.cursor)
	m.review.selection = &diffSelection
	m.move(forward)
	m.review.viewport = m.review.view.scrollHorizontal(m.review.viewport, 12)
	oldView := m.review.view
	wantCursor, wantViewport, wantSelection := m.review.cursor, m.review.viewport, *m.review.selection
	m.err = fmt.Errorf("previous refresh failed")

	m = updateModel(t, m, refreshDiffMsg{patch: p})
	if m.review.view == oldView {
		t.Fatal("unchanged refresh did not rebuild")
	}
	if m.review.cursor != wantCursor || m.review.viewport != wantViewport || m.review.selection == nil || *m.review.selection != wantSelection {
		t.Fatalf("viewState after refresh: diffCursor=%#v diffViewport=%#v diffSelection=%#v", m.review.cursor, m.review.viewport, m.review.selection)
	}
	if m.err != nil {
		t.Fatalf("refresh did not clear error: %v", m.err)
	}
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	oldView, oldCursor := m.review.view, m.review.cursor
	m = updateModel(t, m, refreshDiffMsg{err: fmt.Errorf("git failed")})
	if m.err == nil || m.review.view != oldView || m.review.cursor != oldCursor || m.review.patch.Root != "/repo" {
		t.Fatalf("view=%v diffCursor=%#v patch=%#v error=%v", m.review.view, m.review.cursor, m.review.patch, m.err)
	}
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.width = 120
	m.move(1)
	m.move(1)
	want, _ := m.review.view.line(m.review.cursor)
	oldCoordinate := m.review.cursor.coordinate
	m.setSideBySide(true)
	got, ok := m.review.view.line(m.review.cursor)
	if !ok || got != want {
		t.Fatalf("diffCursor line after switch = %#v", got)
	}
	if m.review.cursor.coordinate == oldCoordinate {
		t.Fatal("layout switch reused the old coordinate")
	}
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := testModel(modelPatch(), nil, func(stored comments.Comment, p patch.Patch) (comments.Comment, error) {
		savedPatch = p
		stored.ID = "1"
		return stored, nil
	})
	m.comments.body = "comment"
	m.comments.editAnchor = comments.Anchor{FilePath: "main.go"}
	m.finishCommentEdit()
	if savedPatch.Root != "/repo" || len(m.comments.items) != 1 || m.comments.items[0].Anchor.FilePath != "main.go" {
		t.Fatalf("saved patch/comments = %#v %#v", savedPatch, m.comments.items)
	}
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.width, m.height = 80, 10
	m.review.viewport = m.review.view.resize(m.review.viewport, m.width, m.screenBodyHeight())
	rendered := m.render()
	for _, value := range []string{"review-my-slop", "+1-1", "old()", "new()", "local changes"} {
		if !strings.Contains(rendered, value) {
			t.Fatalf("render missing %q: %q", value, rendered)
		}
	}
	m.mode = modeHelp
	if !strings.Contains(m.render(), "Ctrl-w h/l/w") {
		t.Fatal("help lost diffPane binding")
	}
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.width, m.height = 80, 10

	lines := strings.Split(m.render(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(got, want) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(got, "No unstaged or untracked changes.") {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := testModel(modelPatch(), []comments.Comment{{Body: "first", Anchor: comments.Anchor{FilePath: "main.go", NewStart: 2}}}, nil)
	m.width, m.height = 80, 10

	tests := []struct {
		name string
		mode mode
		hint string
	}{
		{name: "comments", mode: modeComments, hint: "j/k move"},
		{name: "help", mode: modeHelp, hint: "? or Esc closes help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.mode = test.mode
			lines := strings.Split(m.render(), "\n")
			if got := lines[m.height-2]; !strings.Contains(got, test.hint) {
				t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, test.hint)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{Body: fmt.Sprintf("comment %d", index), Anchor: comments.Anchor{FilePath: "main.go"}}
	}
	m := testModel(modelPatch(), items, nil)
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.comments.row = len(items) - 1

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	if !strings.Contains(strings.Join(rendered[1:m.height-2], "\n"), "comment 9") {
		t.Fatalf("selected comment is outside the screen body: %q", rendered)
	}
	if !strings.Contains(rendered[m.height-2], "j/k move") {
		t.Fatalf("footer line = %q", rendered[m.height-2])
	}
}

func modelPatch() patch.Patch {
	return patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
}
