package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{{ID: "one", Body: "old body"}, {ID: "two", Body: "second"}}
	var persisted, deleted comments.Comment
	m := testModel(coveragePatch(), items, func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
		persisted = stored
		return stored, nil
	})
	m.SetDelete(func(stored comments.Comment, _ patch.Patch) error { deleted = stored; return nil })
	m = updateModel(t, m, textKey("C"))
	if m.screen != screenComments || !strings.Contains(m.render(), "old body") {
		t.Fatal("comments did not open")
	}
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "edited body"})
	if persisted.ID != "one" || persisted.Body != "edited body" {
		t.Fatalf("persisted = %#v", persisted)
	}
	m = updateModel(t, m, textKey("D"))
	if deleted.ID != "one" || len(m.comments.items) != 1 {
		t.Fatalf("deleted=%#v comments=%#v", deleted, m.comments.items)
	}
	m = updateModel(t, m, textKey("q"))
	if m.screen != screenReview || m.quitting {
		t.Fatalf("screen=%v quitting=%v", m.screen, m.quitting)
	}
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "read", Body: "already read"}}, nil)
	m.SetLoadComments(func() ([]comments.Comment, error) { return nil, nil })

	next, cmd := m.Update(textKey("C"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("opening comments did not request a refresh")
	}
	m = updateModel(t, m, cmd())
	if m.screen != screenComments || len(m.comments.items) != 0 {
		t.Fatalf("screen=%v comments=%#v", m.screen, m.comments.items)
	}
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "keep", Body: "keep"}}, nil)
	m.SetLoadComments(func() ([]comments.Comment, error) { return nil, fmt.Errorf("storage unavailable") })

	next, cmd := m.Update(textKey("C"))
	m = next.(Model)
	m = updateModel(t, m, cmd())
	if len(m.comments.items) != 1 || m.err == nil || m.err.Error() != "refresh comments: storage unavailable" {
		t.Fatalf("comments=%#v error=%v", m.comments.items, m.err)
	}
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "old"}}, nil)
	deleted := false
	m.SetDelete(func(comments.Comment, patch.Patch) error { deleted = true; return nil })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "\n"})
	if !deleted || len(m.comments.items) != 0 {
		t.Fatalf("deleted=%v comments=%d", deleted, len(m.comments.items))
	}
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := testModel(coveragePatch(), []comments.Comment{{ID: "one", Body: "keep"}}, nil)
	m.SetDelete(func(comments.Comment, patch.Patch) error { return fmt.Errorf("delete failed") })
	m = updateModel(t, m, textKey("C"))
	m = updateModel(t, m, textKey("D"))
	if len(m.comments.items) != 1 || !strings.Contains(ansi.Strip(m.renderComments()), "delete failed") {
		t.Fatalf("comments=%#v render=%q", m.comments.items, m.renderComments())
	}
}
