package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func testModel(
	p patch.Patch,
	items []comments.Comment,
	save saveCommentFunc,
) model {
	return newReviewModel(
		p,
		items,
		save,
		initialLayout{size: size{Width: 100, Height: 30}},
	)
}

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newModelWithStore(store, modelPatch(), nil, defaultSize)
	testutil.RequireNoError(t, err)

	anchor := comments.Anchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	m.saveEditedComment()
	testutil.RequireValuesEqual(t, m.err, nil, "%s", m.err)

	loaded := loadStoredComments(t, m)
	requireSavedComment(t, loaded, "", "check this", anchor)

	id := loaded[0].ID
	m.edit.index = 0
	m.edit.body = "edited"
	m.saveEditedComment()
	testutil.RequireValuesEqual(t, m.err, nil, "%s", m.err)

	loaded = loadStoredComments(t, m)
	requireSavedComment(t, loaded, id, "edited", anchor)

	m.deleteComment(0)
	testutil.RequireValuesEqual(t, m.err, nil, "%s", m.err)

	loaded = loadStoredComments(t, m)
	testutil.RequireValuesEqual(
		t,
		len(loaded),
		0,
		"remaining = %#v, error = %v",
		loaded,
		err,
	)
}

func loadStoredComments(t *testing.T, m model) []comments.Comment {
	t.Helper()
	loaded, err := m.load()
	testutil.RequireNoError(t, err)
	return loaded
}

func requireSavedComment(
	t *testing.T,
	loaded []comments.Comment,
	wantID, wantBody string,
	wantAnchor comments.Anchor,
) {
	t.Helper()
	testutil.RequireValuesEqual(t, len(loaded), 1, "loaded = %#v", loaded)
	comment := loaded[0]
	testutil.RequireValuesEqual(
		t, comment.Repository, "/repo", "comment = %#v", comment,
	)
	testutil.RequireValuesEqual(
		t, comment.Body, wantBody, "comment = %#v", comment,
	)
	if wantID == "" {
		testutil.RequireValuesEqual(
			t, comment.ID != "", true, "comment = %#v", comment,
		)
	} else {
		testutil.RequireValuesEqual(
			t, comment.ID, wantID, "comment = %#v", comment,
		)
	}
	testutil.RequireValuesEqual(
		t,
		slices.Equal(comment.Anchor.QuotedLines, wantAnchor.QuotedLines),
		true,
		"comment = %#v",
		comment,
	)
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newReviewModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 120, Height: 30},
	})
	testutil.RequireValuesEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(m.formatCurrentScreen(), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newReviewModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 80, Height: 30},
	})
	testutil.RequireValuesEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(m.formatCurrentScreen(), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireValuesEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(m.formatCurrentScreen(), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		m.formatCurrentScreen(),
	)
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newReviewModel(modelPatch(), nil, nil, initialLayout{
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		size: size{Width: 120, Height: 30},
	})

	m = updateModel(t, m, textKey("t"))
	updateModel(t, m, textKey("t"))
	testutil.RequireValuesEqual(
		t,
		slices.Equal(saved, []bool{true, false}),
		true,
		"saved=%v",
		saved,
	)
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	_, want, _ := m.diffView.FocusedFileAndLine()
	refreshed := modelPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.showChangedFiles(refreshed)
	_, got, ok := m.diffView.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
	testutil.RequireValuesEqual(
		t,
		got,
		want,
		"focused line=%#v, want %#v",
		got,
		want,
	)

	_, lines, ok := m.diffView.SelectedFileAndLines()
	testutil.RequireValuesEqual(t, ok, true, "selection=%#v", lines)
	testutil.RequireValuesEqual(t, len(lines), 2, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longModelPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newReviewModel(
		p,
		nil,
		nil,
		initialLayout{size: size{Width: 40, Height: 8}},
	)
	for range 8 {
		m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.FormatReviewScreen(nil)
	m.err = errors.New("previous refresh failed")
	m = updateModel(t, m, refreshDiffMsg{patch: p})
	if got := m.diffView.FormatReviewScreen(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	testutil.RequireValuesEqual(
		t,
		m.err,
		nil,
		"refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	before := m.diffView.FormatReviewScreen(nil)
	m = updateModel(t, m, refreshDiffMsg{err: errors.New("git failed")})
	testutil.RequireValuesEqual(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	testutil.RequireValuesEqual(t, m.diffView.FormatReviewScreen(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	testutil.RequireValuesEqual(
		t,
		m.currentPatch.Root,
		"/repo",
		"patch=%#v error=%v",
		m.currentPatch,
		m.err,
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	m.diffView.MoveFocusWithinChanges(diffscreen.NextLine)
	_, want, _ := m.diffView.FocusedFileAndLine()
	m.setSplitView(true)
	_, got, ok := m.diffView.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
	testutil.RequireValuesEqual(
		t,
		got,
		want,
		"focused line=%#v, want %#v",
		got,
		want,
	)
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := testModel(modelPatch(), nil, func(
		stored comments.Comment,
		p patch.Patch,
	) (comments.Comment, error) {
		savedPatch = p
		stored.ID = "1"
		return stored, nil
	})
	m.edit.body = "comment"
	m.edit.anchor = comments.Anchor{FilePath: "main.go"}
	m.saveEditedComment()
	testutil.RequireValuesEqual(t, savedPatch.Root, "/repo",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireValuesEqual(t, len(m.comments.items), 1,
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireValuesEqual(
		t,
		m.comments.items[0].Anchor.FilePath,
		"main.go",
		"saved patch/comments = %#v %#v",
		savedPatch,
		m.comments.items,
	)
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.width, m.height = 80, 10
	m.resizeViews()
	m.resizeViews()
	rendered := m.formatCurrentScreen()
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		testutil.RequireValuesEqual(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = modeHelp
	testutil.RequireValuesEqual(
		t,
		strings.Contains(m.formatCurrentScreen(), "Ctrl-w h/l/w"),
		true,
		"%s",
		"help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.width, m.height = 80, 10
	m.resizeViews()

	lines := strings.Split(m.formatCurrentScreen(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(
		got,
		want,
	) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(
		got,
		"No unstaged or untracked changes.",
	) {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := testModel(
		modelPatch(),
		[]comments.Comment{
			{
				Body:   "first",
				Anchor: comments.Anchor{FilePath: "main.go", NewStart: 2},
			},
		},
		nil,
	)
	m.width, m.height = 80, 10
	m.resizeViews()

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
			lines := strings.Split(m.formatCurrentScreen(), "\n")
			if got := lines[m.height-2]; !strings.Contains(got, test.hint) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.height-1,
					got,
					test.hint,
				)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: comments.Anchor{FilePath: "main.go"},
		}
	}
	m := testModel(modelPatch(), items, nil)
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.resizeViews()
	m.commentView.MoveCommentSelection(len(items))

	rendered := strings.Split(ansi.Strip(m.formatCurrentScreen()), "\n")
	testutil.RequireValuesEqual(t,
		strings.Contains(
			strings.Join(rendered[1:m.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(rendered[m.height-2], "j/k move"),
		true,
		"footer line = %q",
		rendered[m.height-2],
	)
}

func modelPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,2 +1,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
}

func TestNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := newReviewModel(modelPatch(), nil, nil, initialLayout{
		size: size{Width: 80, Height: 30},
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	})
	m = updateModel(t, m, textKey("t"))
	testutil.RequireValuesEqual(
		t,
		m.err,
		nil,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		m.formatCurrentScreen(),
	)
	testutil.RequireValuesEqual(
		t,
		slices.Equal(saved, []bool{true}),
		true,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		m.formatCurrentScreen(),
	)
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(m.formatCurrentScreen(), " │ "),
		true,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		m.formatCurrentScreen(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireValuesEqual(
		t,
		strings.Contains(m.formatCurrentScreen(), " │ "),
		true,
		"%s",
		"widening did not activate preferred layout",
	)
}
