package app

import (
	"context"
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
	return newModel(
		context.Background(),
		p,
		items,
		save,
		initialLayout{dimensions: terminalDimensions{width: 100, height: 30}},
	)
}

func TestBrowseKeyDispatch(t *testing.T) {
	for _, tt := range []struct {
		name string
		keys []string
		mode mode
	}{
		{name: "help", keys: []string{"?"}, mode: modeHelp},
		{name: "search", keys: []string{"/"}, mode: modeSearch},
		{name: "comments", keys: []string{"C"}, mode: modeComments},
		{name: "unknown", keys: []string{"unknown"}, mode: modeBrowse},
		{name: "pending prefix", keys: []string{"z"}, mode: modeBrowse},
		{
			name: "consumed continuation",
			keys: []string{"z", "?"}, mode: modeBrowse,
		},
		{
			name: "retried continuation",
			keys: []string{"g", "?"}, mode: modeHelp,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(modelPatch(), nil, nil)
			m.err = errors.New("previous error")
			for _, key := range tt.keys {
				m = updateModel(t, m, textKey(key))
			}
			testutil.RequireEqual(t, m.mode, tt.mode, "mode = %v", m.mode)
			testutil.RequireEqual(t, m.err, nil, "error = %v", m.err)
		})
	}
}

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newWithStore(
		context.Background(),
		store,
		modelPatch(),
		nil,
		defaultDimensions,
	)
	testutil.NoError(t, err)

	anchor := comments.Anchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.anchor = anchor
	m.finishCommentEdit("check this")
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err := m.load()
	testutil.RequireEqual(t, err, nil, "loaded = %#v, error = %v", loaded, err)
	testutil.RequireEqual(t, len(loaded), 1, "loaded = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, loaded[0].Repository, "/repo",
		"loaded = %#v, error = %v", loaded, err,
	)
	testutil.RequireEqual(t, loaded[0].ID != "", true,
		"loaded = %#v, error = %v", loaded, err,
	)
	testutil.RequireEqual(t,
		slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines), true,
		"loaded = %#v, error = %v", loaded, err,
	)

	id := loaded[0].ID
	m.edit.index = 0
	m.finishCommentEdit("edited")
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil, "edited = %#v, error = %v", loaded, err)
	testutil.RequireEqual(t, len(loaded), 1, "edited = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, loaded[0].ID, id, "edited = %#v, error = %v",
		loaded, err,
	)
	testutil.RequireEqual(t, loaded[0].Body, "edited",
		"edited = %#v, error = %v", loaded, err,
	)

	m.deleteComment(0)
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil, "remaining = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, len(loaded), 0, "remaining = %#v, error = %v",
		loaded, err,
	)
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		sideBySide: true,
		dimensions: terminalDimensions{width: 120, height: 30},
	})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		sideBySide: true,
		dimensions: terminalDimensions{width: 80, height: 30},
	})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, m.render(),
	)
}

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		saveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		dimensions: terminalDimensions{width: 120, height: 30},
	})

	m = updateModel(t, m, textKey("t"))
	updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	refreshed := modelPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.updatePatch(refreshed)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)

	_, lines, ok := m.diffView.Selected()
	testutil.RequireEqual(t, ok, true, "selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longModelPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newModel(
		context.Background(),
		p,
		nil,
		nil,
		initialLayout{dimensions: terminalDimensions{width: 40, height: 8}},
	)
	for range 8 {
		m.diffView.Move(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = updateModel(t, m, refreshDiffMsg{patch: p})
	if got := m.diffView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	testutil.RequireEqual(t, m.err, nil, "refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	before := m.diffView.Render(nil)
	m = updateModel(t, m, refreshDiffMsg{err: errors.New("git failed")})
	testutil.RequireEqual(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.diffView.Render(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.currentPatch.Root, "/repo", "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	m.setSideBySide(true)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)
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
	m.edit.anchor = comments.Anchor{FilePath: "main.go"}
	m.finishCommentEdit("comment")
	testutil.RequireEqual(t, savedPatch.Root, "/repo",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireEqual(t, m.comments.items[0].Anchor.FilePath, "main.go",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()
	m.resizeScreens()
	rendered := m.render()
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		testutil.RequireEqual(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = modeHelp
	testutil.RequireEqual(t, strings.Contains(m.render(), "Ctrl-w h/l/w"), true,
		"%s", "help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()

	lines := strings.Split(m.render(), "\n")
	got, want := lines[m.dimensions.height-2], "j/k/h/l move"
	if !strings.Contains(got, want) {
		t.Fatalf(
			"line %d = %q, want it to contain %q",
			m.dimensions.height-1,
			got,
			want,
		)
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
	m.dimensions = terminalDimensions{width: 80, height: 10}
	m.resizeScreens()

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
			if got := lines[m.dimensions.height-2]; !strings.Contains(
				got,
				test.hint,
			) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.dimensions.height-1,
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
	m.dimensions = terminalDimensions{width: 80, height: 7}
	m.mode = modeComments
	m.resizeScreens()
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(m.render()), "\n")
	testutil.RequireEqual(t,
		strings.Contains(
			strings.Join(rendered[1:m.dimensions.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	testutil.RequireEqual(
		t,
		strings.Contains(rendered[m.dimensions.height-2], "j/k move"),
		true, "footer line = %q", rendered[m.dimensions.height-2],
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
	m := newModel(context.Background(), modelPatch(), nil, nil, initialLayout{
		dimensions: terminalDimensions{width: 80, height: 30},
		saveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	})
	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, m.err, nil,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true}), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)
	testutil.RequireEqual(t, !strings.Contains(m.render(), " │ "), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, m.render(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	testutil.RequireEqual(t, strings.Contains(m.render(), " │ "), true, "%s",
		"widening did not activate preferred layout",
	)
}
