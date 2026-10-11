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
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func testModel(
	p patch.Patch,
	items []comments.Comment,
	save saveCommentFunc,
) model {
	return newModel(
		p,
		items,
		save,
		initialLayout{size: size{Width: 100, Height: 30}},
	)
}

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newWithStore(store, modelPatch(), nil, defaultSize)
	requireEqual(t, err == nil, true, valueFormat, err)
	anchor := comments.Anchor{
		FilePath:    fixtureMainGo,
		NewStart:    two,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	m.finishCommentEdit()
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err := m.load()
	requireEqual(

		t,

		err == nil && len(loaded) == one &&
			loaded[zero].Repository == "/repo" &&

			loaded[zero].ID != "" &&

			slices.Equal(loaded[zero].Anchor.QuotedLines, anchor.QuotedLines),

		true,

		"loaded = %#v, error = %v",

		loaded,

		err,
	)
	id := loaded[zero].ID
	m.edit.index = zero
	m.edit.body = "edited"
	m.finishCommentEdit()
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err = m.load()
	requireEqual(
		t,
		err == nil && len(loaded) == one && loaded[zero].ID == id &&
			loaded[zero].Body == "edited",
		true,
		"edited = %#v, error = %v",
		loaded,
		err,
	)
	m.deleteComment(zero)
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err = m.load()
	requireEqual(
		t,
		err == nil && len(loaded) == zero, true,
		"remaining = %#v, error = %v",
		loaded,
		err,
	)
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := newModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: 120, Height: 30},
	})
	requireEqual(
		t,
		m.diffOptions.SideBySide && strings.Contains(m.render(), "│"),
		true,
		fixtureSideBySideVRenderQ,
		m.diffOptions.SideBySide,
		m.render(),
	)
}

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := newModel(modelPatch(), nil, nil, initialLayout{
		SideBySide: true,
		size:       size{Width: defaultWidth, Height: 30},
	})
	requireEqual(
		t,
		m.diffOptions.SideBySide && !strings.Contains(m.render(), "│"),
		true,
		fixtureSideBySideVRenderQ,
		m.diffOptions.SideBySide,
		m.render(),
	)

	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	requireEqual(
		t,
		m.diffOptions.SideBySide && strings.Contains(m.render(), "│"),
		true,
		fixtureSideBySideVRenderQ,
		m.diffOptions.SideBySide,
		m.render(),
	)
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

	m = updateModel(t, m, textKey(fixtureT))
	updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		slices.Equal(saved, []bool{true, false}), true,
		"saved=%v",
		saved,
	)
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	refreshed := modelPatch()
	refreshed.Files[zero].Metadata = []string{"new metadata"}
	m.updatePatch(refreshed)
	_, got, ok := m.diffView.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
	_, lines, ok := m.diffView.Selected()
	requireEqual(t, ok && len(lines) == two, true, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := longModelPatch()
	p.Files[zero].OldPath, p.Files[zero].NewPath = "long.go", "long.go"
	m := newModel(
		p,
		nil,
		nil,
		initialLayout{size: size{Width: 40, Height: eight}},
	)
	for range eight {
		m.diffView.Move(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(horizontalScrollColumns)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = updateModel(t, m, refreshDiffMsg{patch: p})
	{
		got := m.diffView.Render(nil)
		requireEqual(
			t,
			got, before,
			valueFormat,
			"refresh changed presentation state",
		)
	}
	requireEqual(
		t,
		m.err == nil, true,
		"refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	before := m.diffView.Render(nil)
	m = updateModel(t, m, refreshDiffMsg{err: errors.New("git failed")})
	requireEqual(
		t,
		m.currentPatch.Root == "/repo" &&
			(m.err != nil && m.diffView.Render(nil) == before),
		true,
		"patch=%#v error=%v",
		m.currentPatch,
		m.err,
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
	requireEqual(
		t,
		ok && got == want, true,
		"focused line=%#v, want %#v",
		got,
		want,
	)
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := testModel(
		modelPatch(),
		nil,
		func(
			stored comments.Comment,
			p patch.Patch,
		) (
			comments.Comment,
			error,
		) {
			savedPatch = p
			stored.ID = "1"
			return stored, nil
		},
	)
	m.edit.body = "comment"
	m.edit.anchor = comments.Anchor{FilePath: fixtureMainGo}
	m.finishCommentEdit()
	requireEqual(
		t,
		savedPatch.Root == "/repo" && len(m.comments.items) == one &&
			m.comments.items[zero].Anchor.FilePath == fixtureMainGo,
		true,
		"saved patch/comments = %#v %#v",
		savedPatch,
		m.comments.items,
	)
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := testModel(modelPatch(), nil, nil)
	m.width, m.height = defaultWidth, ten
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
		requireEqual(
			t,
			strings.Contains(rendered, value), true,
			"render missing %q: %q",
			value,
			rendered,
		)
	}
	m.mode = modeHelp
	requireEqual(
		t,
		strings.Contains(m.render(), "Ctrl-w h/l/w"),
		true,
		valueFormat,
		"help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := testModel(patch.Patch{}, nil, nil)
	m.width, m.height = defaultWidth, ten
	m.resizeScreens()

	lines := strings.Split(m.render(), newline)
	{
		got, want := lines[m.height-two], "j/k/h/l move"
		requireEqual(
			t,
			strings.Contains(got, want),
			true,
			"line %d = %q, want it to contain %q",
			m.height-one,
			got,
			want,
		)
	}
	{
		got := lines[two]
		requireEqual(
			t,
			strings.Contains(got, "No unstaged or untracked changes."),
			true,
			"empty-viewState line = %q",
			got,
		)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := testModel(
		modelPatch(),
		[]comments.Comment{
			{
				Body: "first",
				Anchor: comments.Anchor{
					FilePath: fixtureMainGo,
					NewStart: two,
				},
			},
		},
		nil,
	)
	m.width, m.height = defaultWidth, ten
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
		t.Run(
			test.name,
			func(t *testing.T) {
				m.mode = test.mode
				lines := strings.Split(m.render(), newline)
				{
					got := lines[m.height-two]
					requireEqual(
						t,
						strings.Contains(got, test.hint),
						true,
						"line %d = %q, want it to contain %q",
						m.height-one,
						got,
						test.hint,
					)
				}
			},
		)
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, ten)
	for index := range items {
		items[index] = comments.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: comments.Anchor{FilePath: fixtureMainGo},
		}
	}
	m := testModel(modelPatch(), items, nil)
	m.width, m.height = defaultWidth, seven
	m.mode = modeComments
	m.resizeScreens()
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(m.render()), newline)
	requireEqual(
		t,
		strings.Contains(
			strings.Join(rendered[one:m.height-two], newline),
			"comment 9",
		),
		true,
		"selected comment is outside the screen body: %q",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[m.height-two], "j/k move"), true,
		"footer line = %q",
		rendered[m.height-two],
	)
}

func modelPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: fixtureMainGo,
				OldPath:     fixtureMainGo,
				NewPath:     fixtureMainGo,
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,2 +1,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: one,
								NewNumber: one,
							},
							{
								Kind:      patch.Deletion,
								Text:      "old()",
								OldNumber: two,
							},
							{
								Kind:      patch.Addition,
								Text:      "new()",
								NewNumber: two,
							},
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
	m := newModel(
		modelPatch(),
		nil,
		nil,
		initialLayout{
			size: size{Width: defaultWidth, Height: 30},
			SaveSideBySide: func(enabled bool) error {
				saved = append(saved, enabled)
				return nil
			},
		},
	)
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		m.err == nil && slices.Equal(saved, []bool{true}) &&
			!strings.Contains(m.render(), " │ "),
		true,
		"narrow toggle: saved=%v err=%v render=%q",
		saved,
		m.err,
		m.render(),
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	requireEqual(
		t,
		strings.Contains(m.render(), " │ "),
		true,
		valueFormat,
		"widening did not activate preferred layout",
	)
}
