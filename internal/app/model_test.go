package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
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
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	m.finishCommentEdit()
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err := m.load()
	requireEqual(

		t,

		err == nil && len(loaded) == 1 &&
			loaded[0].Repository == "/repo" &&

			loaded[0].ID != "" &&

			slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines),

		true,

		"loaded = %#v, error = %v",

		loaded,

		err,
	)
	id := loaded[0].ID
	m.edit.index = 0
	m.edit.body = "edited"
	m.finishCommentEdit()
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err = m.load()
	requireEqual(
		t,
		err == nil && len(loaded) == 1 && loaded[0].ID == id &&
			loaded[0].Body == "edited",
		true,
		"edited = %#v, error = %v",
		loaded,
		err,
	)
	m.deleteComment(0)
	requireEqual(t, m.err == nil, true, valueFormat, m.err)
	loaded, err = m.load()
	requireEqual(
		t,
		err == nil && len(loaded) == 0, true,
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
								OldNumber: 1,
								NewNumber: 1,
							},
							{
								Kind:      patch.Deletion,
								Text:      "old()",
								OldNumber: 2,
							},
							{
								Kind:      patch.Addition,
								Text:      "new()",
								NewNumber: 2,
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
