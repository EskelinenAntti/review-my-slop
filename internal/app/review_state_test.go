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
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func TestAppStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := feedback.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newStoreBackedTestAppModel(store)
	requireNoError(t, err)

	anchor := feedback.Anchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	id := assertTestStoredCommentCreated(t, &m, anchor)
	assertTestStoredCommentUpdated(t, &m, id)
	assertTestStoredCommentDeleted(t, &m)
}

func assertTestStoredCommentCreated(
	t *testing.T,
	model *reviewModel,
	anchor feedback.Anchor,
) string {
	t.Helper()
	model.edit.body = "check this"
	model.edit.anchor = anchor
	model.finishCommentEditor(commentEditorFinishedMsg{
		body: model.edit.body,
	})
	requireEqual(t, model.err, nil, "%s", model.err)
	loaded, err := model.load()
	requireEqual(t, err, nil, "loaded = %#v, error = %v", loaded, err)
	requireEqual(t, len(loaded), 1, "loaded = %#v, error = %v", loaded, err)
	requireEqual(t, loaded[0].Repository, "/repo",
		"loaded = %#v, error = %v", loaded, err)
	requireEqual(t, loaded[0].ID != "", true,
		"loaded = %#v, error = %v", loaded, err)
	requireEqual(t,
		slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines),
		true, "loaded = %#v, error = %v", loaded, err)
	return loaded[0].ID
}

func assertTestStoredCommentUpdated(
	t *testing.T,
	model *reviewModel,
	id string,
) {
	t.Helper()
	model.edit.index = 0
	model.edit.body = "edited"
	model.finishCommentEditor(commentEditorFinishedMsg{
		body: model.edit.body,
	})
	requireEqual(t, model.err, nil, "%s", model.err)
	loaded, err := model.load()
	requireEqual(t, err, nil, "edited = %#v, error = %v", loaded, err)
	requireEqual(t, len(loaded), 1, "edited = %#v, error = %v", loaded, err)
	requireEqual(t, loaded[0].ID, id, "edited = %#v, error = %v", loaded, err)
	requireEqual(t, loaded[0].Body, "edited",
		"edited = %#v, error = %v", loaded, err)
}

func assertTestStoredCommentDeleted(t *testing.T, model *reviewModel) {
	t.Helper()
	model.edit.index = 0
	model.edit.body = ""
	model.finishCommentEditor(commentEditorFinishedMsg{
		body: model.edit.body,
	})
	requireEqual(t, model.err, nil, "%s", model.err)
	loaded, err := model.load()
	requireEqual(t, err, nil, "remaining = %#v, error = %v", loaded, err)
	requireEqual(t, len(loaded), 0, "remaining = %#v, error = %v", loaded, err)
}

func newStoreBackedTestAppModel(store feedback.Store) (reviewModel, error) {
	patch := patches.Patch{Root: "/repo", Files: []patches.File{{
		DisplayPath: "main.go",
		OldPath:     "main.go",
		NewPath:     "main.go",
		Hunks: []patches.Hunk{{
			Header: "@@ -1,2 +1,2 @@",
			Lines: []patches.Line{{
				Kind: patches.Context, Text: "keep()", OldNumber: 1,
			}, {
				Kind: patches.Deletion, Text: "old()", OldNumber: 2,
			}, {
				Kind: patches.Addition, Text: "new()", NewNumber: 2,
			}},
		}},
	}}}
	preferences, err := readTestUIPreferences()
	if err != nil {
		return reviewModel{}, err
	}
	layout := initialLayout{
		SideBySide:     preferences.SideBySide,
		SaveSideBySide: saveLayoutPreference,
		size:           defaultSize,
	}
	model := newTestAppModelWithLayout(patch, layout)
	model.save = func(
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
		return commentStore(store).Add(comment)
	}
	model.delete = func(comment feedback.Comment, current patches.Patch) error {
		return commentStore(store).Delete(current.Root, comment.ID)
	}
	model.load = func() ([]feedback.Comment, error) {
		return commentStore(store).List(patch.Root)
	}
	return model, nil
}

func TestAppNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newTestAppModelWithLayout(
		newTestBasicPatch(),
		initialLayout{
			SideBySide: true,
			size:       size{Width: 120, Height: 30},
		},
	)
	requireEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderTestAppMode(m),
	)
	requireEqual(
		t,
		strings.Contains(renderTestAppMode(m), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderTestAppMode(m),
	)
}

func TestAppNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newTestAppModelWithLayout(
		newTestBasicPatch(),
		initialLayout{
			SideBySide: true,
			size:       size{Width: 80, Height: 30},
		},
	)
	requireEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderAppWorkflowMode(&m),
	)
	requireEqual(
		t,
		!strings.Contains(renderAppWorkflowMode(&m), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderAppWorkflowMode(&m),
	)

	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	requireEqual(
		t,
		m.diffOptions.SideBySide,
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderAppWorkflowMode(&m),
	)
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "│"),
		true,
		"sideBySide=%v render=%q",
		m.diffOptions.SideBySide,
		renderAppWorkflowMode(&m),
	)
}

func TestAppSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := newTestAppModelWithLayout(newTestBasicPatch(), initialLayout{
		SaveSideBySide: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
		size: size{Width: 120, Height: 30},
	})
	m = updateTestAppWithKey(&m, t)
	updateTestAppWithKey(&m, t)
	requireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestAppRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := newTestAppModelWithLayout(
		testCursorTranslationPatch(),
		initialLayout{size: size{Width: 100, Height: 30}},
	)
	m.diffView.Move(ui.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(ui.NextLine)
	_, want, _ := m.diffView.Current()
	refreshed := testCursorTranslationPatch()
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.currentPatch = refreshed
	m.diffView.Update(refreshed)
	_, got, ok := m.diffView.Current()
	requireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	requireEqual(t, got, want, "focused line=%#v, want %#v", got, want)

	_, lines, ok := m.diffView.Selected()
	requireEqual(t, ok, true, "selection=%#v", lines)
	requireEqual(t, len(lines), 2, "selection=%#v", lines)
}

func testCursorTranslationPatch() patches.Patch {
	return patches.Patch{Root: "/repo", Files: []patches.File{{
		DisplayPath: "main.go",
		OldPath:     "main.go",
		NewPath:     "main.go",
		Hunks: []patches.Hunk{{
			Header: "@@ -1,2 +1,2 @@",
			Lines: []patches.Line{{
				Kind: patches.Context, Text: "keep()", OldNumber: 1,
			}, {
				Kind: patches.Deletion, Text: "old()", OldNumber: 2,
			}, {
				Kind: patches.Addition, Text: "new()", NewNumber: 2,
			}},
		}},
	}}}
}

func TestAppUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := makeTestPatchWithManyLines()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := newTestAppModelWithLayout(
		p,
		initialLayout{size: size{Width: 40, Height: 8}},
	)
	for range 8 {
		m.diffView.Move(ui.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(ui.NextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = updateTestAppModel(t, m, refreshDiffMsg{patch: p})
	if got := m.diffView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	requireEqual(t, m.err, nil, "refresh did not clear error: %v", m.err)
}

func TestAppRefreshFailureRetainsView(t *testing.T) {
	m := newTestAppModelWithLayout(
		testCursorTranslationPatch(),
		initialLayout{size: size{Width: 100, Height: 30}},
	)
	before := m.diffView.Render(nil)
	m = updateTestAppModel(t, m, refreshDiffMsg{
		err: errors.New("git failed"),
	})
	requireEqual(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	requireEqual(t, m.diffView.Render(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	requireEqual(t, m.currentPatch.Root, "/repo", "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
}

func TestAppViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := newTestAppModelWithLayout(
		testCursorTranslationPatch(),
		initialLayout{size: size{Width: 100, Height: 30}},
	)
	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.diffView.Move(ui.NextLine)
	m.diffView.Move(ui.NextLine)
	_, want, _ := m.diffView.Current()
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	_, got, ok := m.diffView.Current()
	requireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	requireEqual(t, got, want, "focused line=%#v, want %#v", got, want)
}

func TestAppCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patches.Patch
	m := newTestAppModel(testCursorTranslationPatch())
	m.save = func(
		stored feedback.Comment,
		patch patches.Patch,
	) (feedback.Comment, error) {
		savedPatch = patch
		stored.ID = "1"
		return stored, nil
	}
	m.edit.body = "comment"
	m.edit.anchor = feedback.Anchor{FilePath: "main.go"}
	m.finishCommentEditor(commentEditorFinishedMsg{body: m.edit.body})
	requireEqual(t, savedPatch.Root, "/repo",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	requireEqual(t, len(m.comments.items), 1,
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	requireEqual(t, m.comments.items[0].Anchor.FilePath, "main.go",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
}

func TestAppRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := newTestAppModel(testCursorTranslationPatch())
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	rendered := renderAppWorkflowMode(&m)
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		requireEqual(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = modeHelp
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), "Ctrl-w h/l/w"),
		true,
		"%s",
		"help lost diffPane binding",
	)
}

func TestAppEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := newTestAppModelWithLayout(
		patches.Patch{},
		initialLayout{size: size{Width: 100, Height: 30}},
	)
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	lines := strings.Split(renderAppWorkflowMode(&m), "\n")
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

func TestAppMenuKeyboardHintsStayAtBottom(t *testing.T) {
	items := []feedback.Comment{{
		Body:   "first",
		Anchor: feedback.Anchor{FilePath: "main.go", NewStart: 2},
	}}
	m := newTestBasicAppModel()
	m.comments = commentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	tests := []struct {
		name             string
		inlineLocalMode1 mode
		hint             string
	}{
		{name: "comments", inlineLocalMode1: modeComments, hint: "j/k move"},
		{
			name:             "help",
			inlineLocalMode1: modeHelp,
			hint:             "? or Esc closes help",
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				m.mode = test.inlineLocalMode1
				assertTestMenuHintAtBottom(t, &m, test.hint)
			},
		)
	}
}

func assertTestMenuHintAtBottom(t *testing.T, model *reviewModel, hint string) {
	t.Helper()
	lines := strings.Split(renderAppWorkflowMode(model), "\n")
	if got := lines[model.height-2]; !strings.Contains(got, hint) {
		t.Fatalf("line %d = %q, want it to contain %q",
			model.height-1, got, hint)
	}
}

func TestAppCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]feedback.Comment, 10)
	for index := range items {
		items[index] = feedback.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: feedback.Anchor{FilePath: "main.go"},
		}
	}
	m := newTestBasicAppModel()
	m.comments = commentState{items: items}
	m.commentView = newTestCommentScreen(items)
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(renderAppWorkflowMode(&m)), "\n")
	requireEqual(t,
		strings.Contains(
			strings.Join(rendered[1:m.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	requireEqual(t, strings.Contains(rendered[m.height-2], "j/k move"),
		true, "footer line = %q", rendered[m.height-2],
	)
}

func TestAppNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := newTestAppModelWithLayout(
		testCursorTranslationPatch(),
		initialLayout{
			size: size{Width: 80, Height: 30},
			SaveSideBySide: func(enabled bool) error {
				saved = append(saved, enabled)
				return nil
			},
		},
	)
	m = updateTestAppWithKey(&m, t)
	requireEqual(
		t,
		m.err,
		nil,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		renderAppWorkflowMode(&m),
	)
	requireEqual(
		t,
		slices.Equal(saved, []bool{true}),
		true,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		renderAppWorkflowMode(&m),
	)
	requireEqual(
		t,
		!strings.Contains(renderAppWorkflowMode(&m), " │ "),
		true,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		renderAppWorkflowMode(&m),
	)

	m = updateTestAppModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	requireEqual(
		t,
		strings.Contains(renderAppWorkflowMode(&m), " │ "),
		true,
		"%s",
		"widening did not activate preferred layout",
	)
}
