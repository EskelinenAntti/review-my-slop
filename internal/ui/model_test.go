package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func TestCommentingASelectionSavesItsSemanticAnchor(t *testing.T) {
	editor := &fakeEditor{bodies: []string{"fix both lines"}}
	var saved []comment.Comment
	model := newTestModel(editor, ui.Dependencies{
		SaveComment: func(item comment.Comment, _ diff.ChangeSet) (comment.Comment, error) {
			saved = append(saved, item)
			item.ID = "new"
			return item, nil
		},
	})

	model = sendKey(t, model, "j")
	model = sendKey(t, model, "v")
	model = sendKey(t, model, "j")
	command := sendKeyCommand(t, &model, "c")
	finishCommand(t, &model, command)

	if len(saved) != 1 {
		t.Fatalf("saved comments = %d", len(saved))
	}
	want := comment.Anchor{FilePath: "main.go", OldStart: 2, OldEnd: 2, NewStart: 2, NewEnd: 2, QuotedLines: []string{"-old()", "+new()"}}
	if got := saved[0].Anchor; got.FilePath != want.FilePath || got.OldStart != want.OldStart || got.NewStart != want.NewStart || !equalQuoted(got.QuotedLines, want.QuotedLines) {
		t.Fatalf("anchor = %#v, want %#v", got, want)
	}
}

func TestEmptyNewCommentDoesNotCallStorage(t *testing.T) {
	editor := &fakeEditor{bodies: []string{" \n"}}
	saved := false
	model := newTestModel(editor, ui.Dependencies{SaveComment: func(comment.Comment, diff.ChangeSet) (comment.Comment, error) {
		saved = true
		return comment.Comment{}, nil
	}})
	command := sendKeyCommand(t, &model, "c")
	finishCommand(t, &model, command)
	if saved {
		t.Fatal("empty comment was persisted")
	}
}

func TestCommentMenuEditsAndDeletesThroughDependencies(t *testing.T) {
	editor := &fakeEditor{bodies: []string{"edited body"}}
	var persisted, deleted comment.Comment
	model := newTestModel(editor, ui.Dependencies{
		SaveComment: func(item comment.Comment, _ diff.ChangeSet) (comment.Comment, error) {
			persisted = item
			return item, nil
		},
		DeleteComment: func(item comment.Comment, _ diff.ChangeSet) error {
			deleted = item
			return nil
		},
	}, comment.Comment{ID: "one", Body: "old body"}, comment.Comment{ID: "two", Body: "second"})
	model = sendKey(t, model, "C")
	command := sendKeyCommand(t, &model, "enter")
	finishCommand(t, &model, command)
	if persisted.ID != "one" || persisted.Body != "edited body" {
		t.Fatalf("persisted = %#v", persisted)
	}
	model = sendKey(t, model, "D")
	if deleted.ID != "one" || !strings.Contains(ansi.Strip(model.View().Content), "1 pending") {
		t.Fatalf("deleted=%#v screen=%q", deleted, model.View().Content)
	}
}

func TestEditingCommentToEmptyDeletesIt(t *testing.T) {
	editor := &fakeEditor{bodies: []string{"\n"}}
	deleted := false
	model := newTestModel(editor, ui.Dependencies{DeleteComment: func(comment.Comment, diff.ChangeSet) error {
		deleted = true
		return nil
	}}, comment.Comment{ID: "one", Body: "old"})
	model = sendKey(t, model, "C")
	command := sendKeyCommand(t, &model, "enter")
	finishCommand(t, &model, command)
	if !deleted || !strings.Contains(ansi.Strip(model.View().Content), "0 pending") {
		t.Fatalf("deleted=%v screen=%q", deleted, model.View().Content)
	}
}

func TestSearchAndHelpAreVisibleUserWorkflows(t *testing.T) {
	model := newTestModel(&fakeEditor{}, ui.Dependencies{})
	model = sendKey(t, model, "/")
	model = sendText(t, model, "new()")
	if !strings.Contains(ansi.Strip(model.View().Content), "new()") {
		t.Fatalf("search prompt missing from screen: %q", model.View().Content)
	}
	model = sendKey(t, model, "enter")
	model = sendKey(t, model, "?")
	help := ansi.Strip(model.View().Content)
	for _, binding := range []string{"Ctrl-w h/l/w", "zz/zt/zb", "]f/[f", "Tab", "t"} {
		if !strings.Contains(help, binding) {
			t.Fatalf("help missing %q", binding)
		}
	}
}

func TestResizeAndLayoutToggleKeepReviewUsable(t *testing.T) {
	model := newTestModel(&fakeEditor{}, ui.Dependencies{})
	model = sendMessage(t, model, tea.WindowSizeMsg{Width: 120, Height: 20})
	model = sendKey(t, model, "t")
	if !strings.Contains(ansi.Strip(model.View().Content), "│") {
		t.Fatal("side-by-side layout did not render")
	}
	model = sendMessage(t, model, tea.WindowSizeMsg{Width: 80, Height: 20})
	if strings.Contains(ansi.Strip(model.View().Content), "│") {
		t.Fatal("narrow layout kept an unusable split")
	}
}

func TestRefreshUsesCurrentChangeSetForNewComment(t *testing.T) {
	editor := &fakeEditor{bodies: []string{"comment"}}
	var saved diff.ChangeSet
	model := newTestModel(editor, ui.Dependencies{
		SaveComment: func(item comment.Comment, changes diff.ChangeSet) (comment.Comment, error) {
			saved = changes
			return item, nil
		},
		RefreshDiff: func(string) (diff.ChangeSet, error) {
			changes := sampleChanges()
			changes.Fingerprint = "refreshed"
			return changes, nil
		},
	})
	command := sendKeyCommand(t, &model, "R")
	finishCommand(t, &model, command)
	command = sendKeyCommand(t, &model, "c")
	finishCommand(t, &model, command)
	if saved.Fingerprint != "refreshed" {
		t.Fatalf("saved fingerprint = %q", saved.Fingerprint)
	}
}

func TestEmptyChangesShowActionableEmptyState(t *testing.T) {
	model := ui.New(diff.ChangeSet{}, nil, ui.Dependencies{}, ui.Layout{Size: ui.Size{Width: 80, Height: 10}})
	screen := ansi.Strip(model.View().Content)
	if !strings.Contains(screen, "No unstaged or untracked changes.") || !strings.Contains(screen, "j/k/h/l move") {
		t.Fatalf("empty screen = %q", screen)
	}
}

func TestEditorFailureIsShownWithoutChangingReviewState(t *testing.T) {
	model := newTestModel(&errorEditor{err: errors.New("editor unavailable")}, ui.Dependencies{})
	model = sendKey(t, model, "c")
	if !strings.Contains(ansi.Strip(model.View().Content), "editor unavailable") {
		t.Fatalf("screen = %q", model.View().Content)
	}
}

func TestSourceEditorReceivesRepositoryPathAndLine(t *testing.T) {
	editor := &fakeEditor{}
	model := newTestModel(editor, ui.Dependencies{})
	command := sendKeyCommand(t, &model, "e")
	if command == nil || len(editor.opened) != 1 || editor.opened[0] != "/repo/main.go:1" {
		t.Fatalf("command=%v opened=%v", command != nil, editor.opened)
	}
}

func TestSaveAndRefreshFailuresRemainVisible(t *testing.T) {
	editor := &fakeEditor{bodies: []string{"comment"}}
	model := newTestModel(editor, ui.Dependencies{
		SaveComment: func(comment.Comment, diff.ChangeSet) (comment.Comment, error) {
			return comment.Comment{}, errors.New("save failed")
		},
	})
	command := sendKeyCommand(t, &model, "c")
	finishCommand(t, &model, command)
	if !strings.Contains(ansi.Strip(model.View().Content), "save failed") {
		t.Fatalf("save error screen = %q", model.View().Content)
	}

	model = newTestModel(&fakeEditor{}, ui.Dependencies{
		RefreshDiff: func(string) (diff.ChangeSet, error) {
			return diff.ChangeSet{}, errors.New("refresh failed")
		},
	})
	command = sendKeyCommand(t, &model, "R")
	finishCommand(t, &model, command)
	if !strings.Contains(ansi.Strip(model.View().Content), "refresh failed") {
		t.Fatalf("refresh error screen = %q", model.View().Content)
	}
}

type errorEditor struct{ err error }

func (e *errorEditor) EditComment(string, comment.Anchor) (tea.Cmd, error) {
	return nil, e.err
}

func (e *errorEditor) OpenSource(string, int) (tea.Cmd, error) {
	return nil, e.err
}
