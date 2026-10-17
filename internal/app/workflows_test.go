package app

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestOpenCommentEditSurvivesListReload(t *testing.T) {
	t.Setenv("EDITOR", "true")
	first := comments.Comment{ID: "one", Body: "first"}
	second := comments.Comment{ID: "two", Body: "second"}
	for _, tc := range []struct {
		name       string
		loaded     []comments.Comment
		saveError  error
		wantBodies []string
	}{
		{"reordered", []comments.Comment{second, first}, nil,
			[]string{"edited body", "first"}},
		{"removed", []comments.Comment{first}, errors.New("comment consumed"),
			[]string{"first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var saved comments.Comment
			m := testModel(coveragePatch(), []comments.Comment{first, second},
				func(comment comments.Comment) (comments.Comment, error) {
					saved = comment
					return comment, tc.saveError
				},
			)
			m = updateModel(t, m, textKey("C"))
			m = updateModel(t, m, textKey("j"))
			m = updateModel(t, m, specialKey(tea.KeyEnter))
			m = updateModel(t, m, commentsLoadedMsg{comments: tc.loaded})
			m = updateModel(t, m,
				commentEditorFinishedMsg{body: "edited body"},
			)
			testutil.RequireEqual(t, saved.ID, second.ID, "saved=%#v", saved)
			testutil.RequireEqual(t, errors.Is(m.err, tc.saveError), true,
				"save error=%v, want %v", m.err, tc.saveError,
			)
			testutil.RequireEqual(t, len(m.comments.items), len(tc.wantBodies),
				"comments=%#v", m.comments.items,
			)
			for index, comment := range m.comments.items {
				testutil.RequireEqual(t, comment.Body, tc.wantBodies[index],
					"comments=%#v", m.comments.items,
				)
			}
		})
	}
}

func TestEmptyEditDeletesOriginalCommentAfterListReload(t *testing.T) {
	t.Setenv("EDITOR", "true")
	first := comments.Comment{ID: "one", Body: "first"}
	second := comments.Comment{ID: "two", Body: "second"}
	m := testModel(coveragePatch(), []comments.Comment{first, second}, nil)
	var deletedID string
	m.delete = func(_ string, id string) error {
		deletedID = id
		return nil
	}
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentsLoadedMsg{
		comments: []comments.Comment{second, first},
	})
	m = updateModel(t, m, commentEditorFinishedMsg{})
	testutil.RequireEqual(t, deletedID, second.ID, "deleted ID=%q", deletedID)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		"comments=%#v", m.comments.items,
	)
	testutil.RequireEqual(t, m.comments.items[0].ID, first.ID,
		"comments=%#v", m.comments.items,
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "read", Body: "already read"}},
		nil,
	)
	m.load = func() ([]comments.Comment, error) { return nil, nil }

	next, cmd := m.Update(textKey("C"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"opening comments did not request a refresh",
	)

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t, m.mode, modeComments, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
}

func TestOlderCommentReloadCannotReplaceNewerComments(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"older success", nil},
		{"older failure", errors.New("older reload failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(coveragePatch(), nil, nil)
			loads := 0
			loadErrors := [2]error{tc.err, nil}
			m.load = func() ([]comments.Comment, error) {
				loads++
				item := comments.Comment{ID: fmt.Sprintf("load-%d", loads)}
				return []comments.Comment{item}, loadErrors[loads-1]
			}
			next, firstCommand := m.Update(textKey("C"))
			m = requireModel(t, next)
			firstResult := firstCommand()
			m = updateModel(t, m, specialKey(tea.KeyEsc))
			next, secondCommand := m.Update(textKey("C"))
			m = requireModel(t, next)
			m = updateModel(t, m, secondCommand())
			m = updateModel(t, m, firstResult)
			testutil.RequireEqual(t, len(m.comments.items), 1,
				"comments=%#v", m.comments.items,
			)
			testutil.RequireEqual(t, m.comments.items[0].ID, "load-2",
				"comments=%#v", m.comments.items,
			)
			testutil.NoError(t, m.err)
		})
	}
}

func TestCommentReloadCannotUndoLocalDeletion(t *testing.T) {
	item := comments.Comment{ID: "one", Body: "pending"}
	m := testModel(coveragePatch(), []comments.Comment{item}, nil)
	m.load = func() ([]comments.Comment, error) {
		return []comments.Comment{item}, nil
	}
	m.delete = func(string, string) error { return nil }
	next, command := m.Update(textKey("C"))
	m = requireModel(t, next)
	loaded := command()
	m = updateModel(t, m, textKey("D"))
	m = updateModel(t, m, loaded)
	testutil.RequireEqual(t, len(m.comments.items), 0,
		"reload restored deleted comments: %#v", m.comments.items,
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(
		coveragePatch(),
		[]comments.Comment{{ID: "keep", Body: "keep"}},
		nil,
	)
	m.load = func() ([]comments.Comment, error) {
		return nil, errors.New("storage unavailable")
	}

	next, cmd := m.Update(textKey("C"))
	m = requireModel(t, next)
	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t, len(m.comments.items), 1, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err != nil, true, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err.Error(),
		"refresh comments: storage unavailable", "comments=%#v error=%v",
		m.comments.items, m.err,
	)
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.refresh = func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{
			fmt.Sprintf("refresh-%d", len(requested)),
		}
		return p, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s", "focus did not refresh")

	m = updateModel(t, m, cmd())
	next, cmd = m.Update(textKey("R"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s", "R did not refresh")

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"refresh-2"},
		), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
}

func TestOlderRefreshCannotReplaceNewerView(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"older success", nil},
		{"older failure", errors.New("older refresh failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(coveragePatch(), nil, nil)
			loads := 0
			loadErrors := [2]error{tc.err, nil}
			m.refresh = func(patch.Kind) (patch.Patch, error) {
				loads++
				p := coveragePatch()
				p.Files[0].Metadata = []string{fmt.Sprintf("load-%d", loads)}
				return p, loadErrors[loads-1]
			}
			next, firstCommand := m.Update(textKey("R"))
			m = requireModel(t, next)
			firstResult := firstCommand()
			next, secondCommand := m.Update(textKey("R"))
			m = requireModel(t, next)
			m = updateModel(t, m, secondCommand())
			m = updateModel(t, m, firstResult)
			testutil.RequireEqual(t,
				slices.Equal(
					m.currentPatch.Files[0].Metadata, []string{"load-2"},
				),
				true, "metadata=%q", m.currentPatch.Files[0].Metadata,
			)
			testutil.NoError(t, m.err)
		})
	}
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := coveragePatch()
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refresh = func(patch.Kind) (patch.Patch, error) { return refreshed, nil }

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"editor completion did not refresh",
	)

	m = updateModel(t, m, cmd())
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true, "metadata=%q", m.currentPatch.Files[0].Metadata,
	)
}
