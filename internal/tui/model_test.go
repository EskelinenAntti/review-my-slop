package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func testModel(p patch.Patch, comments []review.Comment, save SaveCommentFunc) Model {
	return New(Config{Patch: p, Comments: comments, SaveComment: save, Size: Size{Width: 100, Height: 30}})
}

func testModelWith(p patch.Patch, comments []review.Comment, configure func(*Config)) Model {
	config := Config{Patch: p, Comments: comments, Size: Size{Width: 100, Height: 30}}
	if configure != nil {
		configure(&config)
	}
	return New(config)
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := New(Config{Patch: modelPatch(), SideBySide: true, Size: Size{Width: 120, Height: 30}})
	if !m.sideBySide || !m.sideBySideActive() || !strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.sideBySide, m.sideBySideActive(), m.render())
	}
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := New(Config{Patch: modelPatch(), SideBySide: true, Size: Size{Width: 80, Height: 30}})
	if !m.sideBySide || m.sideBySideActive() || strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.sideBySide, m.sideBySideActive(), m.render())
	}

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	if !m.sideBySide || !m.sideBySideActive() || !strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v render=%q", m.sideBySide, m.sideBySideActive(), m.render())
	}
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := New(Config{
		Patch: modelPatch(),
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		Size: Size{Width: 120, Height: 30},
	})

	m = updateModel(t, m, textKey("t"))
	m = updateModel(t, m, textKey("t"))
	if !slices.Equal(saved, []bool{true, false}) {
		t.Fatalf("saved=%v", saved)
	}
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	want := lineText(m)
	refreshed := modelPatch()
	refreshed.Fingerprint = "new"
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m = updateModel(t, m, refreshDiffMsg{patch: refreshed})
	if got := lineText(m); got != want {
		t.Fatalf("cursor line = %q, want %q", got, want)
	}
	if m.review.selection == nil || len(m.review.view.Lines(*m.review.selection)) != 2 {
		t.Fatalf("selection was not translated: %#v", m.review.selection)
	}
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updateModel(t, m, textKey("t"))
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("j"))
	want := lineText(m)
	oldCoordinate := m.review.cursor.Coordinate
	m = updateModel(t, m, textKey("t"))
	if got := lineText(m); got != want {
		t.Fatalf("cursor line after switch = %q, want %q", got, want)
	}
	if m.review.cursor.Coordinate == oldCoordinate {
		t.Fatal("layout switch reused the old coordinate")
	}
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var savedPatch patch.Patch
	m := testModel(modelPatch(), nil, func(stored review.Comment, p patch.Patch) (review.Comment, error) {
		savedPatch = p
		stored.ID = "1"
		return stored, nil
	})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "comment"})
	if savedPatch.Repository != "/repo" || len(m.comments.items) != 1 || m.comments.items[0].Anchor.FilePath != "main.go" {
		t.Fatalf("saved patch/comments = %#v %#v", savedPatch, m.comments.items)
	}
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	rendered := m.render()
	for _, value := range []string{"review-my-slop", "+1-1", "old()", "new()", "local changes"} {
		if !strings.Contains(rendered, value) {
			t.Fatalf("render missing %q: %q", value, rendered)
		}
	}
	m = updateModel(t, m, textKey("?"))
	if !strings.Contains(m.render(), "Ctrl-w h/l/w") {
		t.Fatal("help lost pane binding")
	}
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	const height = 10
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: height})

	lines := strings.Split(m.render(), "\n")
	if got, want := lines[height-2], "j/k/h/l move"; !strings.Contains(got, want) {
		t.Fatalf("line %d = %q, want it to contain %q", height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(got, "No unstaged or untracked changes.") {
		t.Fatalf("empty-state line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	tests := []struct {
		name string
		key  string
		hint string
	}{
		{name: "comments", key: "C", hint: "j/k move"},
		{name: "help", key: "?", hint: "? or Esc closes help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := testModel(modelPatch(), []review.Comment{{Body: "first", Anchor: review.Anchor{FilePath: "main.go", NewStart: 2}}}, nil)
			m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
			m = updateModel(t, m, textKey(test.key))
			lines := strings.Split(m.render(), "\n")
			if got := lines[8]; !strings.Contains(got, test.hint) {
				t.Fatalf("line %d = %q, want it to contain %q", 9, got, test.hint)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	comments := make([]review.Comment, 10)
	for index := range comments {
		comments[index] = review.Comment{Body: fmt.Sprintf("comment %d", index), Anchor: review.Anchor{FilePath: "main.go"}}
	}
	m := testModel(modelPatch(), comments, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 7})
	m = updateModel(t, m, textKey("C"))
	m = moveToLastComment(t, m)

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	if !strings.Contains(strings.Join(rendered[1:5], "\n"), "comment 9") {
		t.Fatalf("selected comment is outside the screen body: %q", rendered)
	}
	if !strings.Contains(rendered[5], "j/k move") {
		t.Fatalf("footer line = %q", rendered[5])
	}
}

func moveToLastComment(t *testing.T, m Model) Model {
	t.Helper()
	for {
		before := m.comments.row
		m = updateModel(t, m, textKey("j"))
		if m.comments.row == before {
			return m
		}
	}
}

func modelPatch() patch.Patch {
	return patch.Patch{
		Repository:  "/repo",
		Fingerprint: "old",
		Files: []patch.File{{
			DisplayPath: "main.go",
			OldPath:     "main.go",
			NewPath:     "main.go",
			Hunks: []patch.Hunk{{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []patch.Line{
					{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1},
					{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
					{Kind: patch.Addition, Text: "new()", NewNumber: 2},
				},
			}},
		}},
	}
}
