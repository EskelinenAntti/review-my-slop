package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func TestNewConstructsWideSideBySideModel(t *testing.T) {
	m := New(testChanges(), nil, Dependencies{}, Options{SideBySide: true, Size: Size{Width: 120, Height: 30}})
	if !m.review.sideBySide || !m.sideBySideActive() || !strings.Contains(m.render(), "│") {
		t.Fatalf("sideBySide=%v active=%v", m.review.sideBySide, m.sideBySideActive())
	}
}

func TestRefreshIgnoresOutOfOrderResults(t *testing.T) {
	var requests []RefreshTarget
	dependencies := Dependencies{RefreshDiff: func(target RefreshTarget) (diff.ChangeSet, error) {
		requests = append(requests, target)
		changes := testChanges()
		changes.Fingerprint = fmt.Sprintf("refresh-%d", len(requests))
		return changes, nil
	}}
	m := New(testChanges(), nil, dependencies, Options{Size: Size{Width: 100, Height: 30}})
	next, first := m.Update(tea.FocusMsg{})
	m = next.(Model)
	next, second := m.Update(textKey("R"))
	m = next.(Model)
	if first == nil || second == nil {
		t.Fatal("refresh commands were not created")
	}
	firstMessage := first()
	secondMessage := second()
	m = updateModel(t, m, secondMessage)
	if m.review.changes.Fingerprint != "refresh-2" {
		t.Fatalf("latest refresh was not applied: %q", m.review.changes.Fingerprint)
	}
	m = updateModel(t, m, firstMessage)
	if m.review.changes.Fingerprint != "refresh-2" {
		t.Fatalf("stale refresh overwrote latest: %q", m.review.changes.Fingerprint)
	}
	if len(requests) != 2 || requests[0] != (RefreshTarget{Mode: LocalChanges}) || requests[1] != (RefreshTarget{Mode: LocalChanges}) {
		t.Fatalf("targets = %#v", requests)
	}
}

func TestBranchRefreshUsesExplicitComparisonMode(t *testing.T) {
	var target RefreshTarget
	m := New(testChanges(), nil, Dependencies{RefreshDiff: func(got RefreshTarget) (diff.ChangeSet, error) {
		target = got
		changes := testChanges()
		changes.Fingerprint = "branch"
		return changes, nil
	}}, Options{DefaultBranch: "main", Size: Size{Width: 100, Height: 30}})
	next, command := m.Update(textKey("tab"))
	m = next.(Model)
	if command == nil {
		t.Fatal("branch toggle did not refresh")
	}
	m = updateModel(t, m, command())
	if target != (RefreshTarget{Mode: BranchChanges, Branch: "main"}) || m.viewMode != BranchChanges {
		t.Fatalf("target=%#v mode=%v", target, m.viewMode)
	}
}

func TestVisualSelectionSavesSemanticAnchor(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved comments.Comment
	m := New(testChanges(), nil, Dependencies{SaveComment: func(comment comments.Comment) (comments.Comment, error) {
		saved = comment
		comment.ID = "new"
		return comment, nil
	}}, Options{Size: Size{Width: 100, Height: 30}})
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("v"))
	m = updateModel(t, m, textKey("j"))
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "fix both lines"})
	if saved.Anchor.FilePath != "first.go" || !slices.Equal(saved.Anchor.QuotedLines, []string{"-removed one", "-removed two"}) {
		t.Fatalf("saved anchor = %#v", saved.Anchor)
	}
	if saved.Anchor.OldStart != 2 || saved.Anchor.OldEnd != 3 {
		t.Fatalf("saved range = %#v", saved.Anchor)
	}
}

func TestCommentSaveFailureDoesNotLeaveDraftState(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := New(testChanges(), nil, Dependencies{SaveComment: func(comments.Comment) (comments.Comment, error) {
		return comments.Comment{}, fmt.Errorf("storage unavailable")
	}}, Options{Size: Size{Width: 100, Height: 30}})
	m = updateModel(t, m, textKey("c"))
	m = updateModel(t, m, commentEditorFinishedMsg{body: "keep this"})
	if m.comments.body != "" || m.err == nil || m.err.Error() != "storage unavailable" {
		t.Fatalf("body=%q error=%v", m.comments.body, m.err)
	}
}

func TestResizeAcrossLayoutThresholdRestoresSemanticCursor(t *testing.T) {
	m := New(testChanges(), nil, Dependencies{}, Options{SideBySide: true, Size: Size{Width: 120, Height: 20}})
	m.review.cursor, _ = m.review.present.Search("added one", m.review.cursor, Forward)
	want, _ := m.review.present.line(m.review.cursor)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = next.(Model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m = next.(Model)
	got, ok := m.review.present.line(m.review.cursor)
	if !ok || got != want {
		t.Fatalf("cursor line=%#v want=%#v", got, want)
	}
}

func updateModel(t *testing.T, model Model, message tea.Msg) Model {
	t.Helper()
	next, _ := model.Update(message)
	result, ok := next.(Model)
	if !ok {
		t.Fatalf("model=%T", next)
	}
	return result
}

func textKey(value string) tea.KeyPressMsg {
	runes := []rune(value)
	return tea.KeyPressMsg(tea.Key{Text: value, Code: runes[0]})
}
