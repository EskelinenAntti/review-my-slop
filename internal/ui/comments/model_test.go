package comments

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	data "github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestReloadRejectsOlderGenerationAndKeepsSelectionByID(t *testing.T) {
	m := New(Initial{Items: []data.Comment{{ID: "selected"}, {ID: "other"}}}, Dependencies{
		List: func() ([]data.Comment, error) { return nil, nil },
	})
	m.row = 1
	older := m.Update(Show{})
	newer := m.Update(Show{})
	apply := func(msg tea.Msg) { m.Update(msg) }
	apply(newer())
	if len(m.items) != 0 {
		t.Fatal("newer empty reload was not applied")
	}
	apply(older())
	if len(m.items) != 0 {
		t.Fatal("older reload overwrote the newer result")
	}

	m.items = []data.Comment{{ID: "a"}, {ID: "b"}}
	m.row = 1
	m.deps.List = func() ([]data.Comment, error) { return []data.Comment{{ID: "b"}, {ID: "a"}}, nil }
	load := m.Update(Show{})
	m.row = 0 // selection moves while the request is in flight
	m.Update(load())
	if m.row != 0 || m.items[m.row].ID != "b" {
		t.Fatalf("selection = %d / %q", m.row, m.items[m.row].ID)
	}
}

func TestReloadResultIsRejectedAfterDeleteMutation(t *testing.T) {
	m := New(Initial{Items: []data.Comment{{ID: "one", Body: "keep"}}}, Dependencies{
		List:   func() ([]data.Comment, error) { return []data.Comment{{ID: "one", Body: "stale"}}, nil },
		Delete: func(string) error { return nil },
	})
	stale := m.Update(Show{})
	deletion := m.delete("one", false)
	m.Update(deletion())
	m.Update(stale())
	if len(m.items) != 0 {
		t.Fatalf("stale reload restored deleted item: %#v", m.items)
	}
}

func TestEditorCapturesAnchorAndBlankNewCompositionCancelsPatchSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	original := patch.Anchor{FilePath: "a.go", QuotedLines: []string{"+return true"}}
	m := New(Initial{}, Dependencies{})
	if cmd := m.Begin(original); cmd == nil {
		t.Fatal("Begin returned no process command")
	}
	path := strings.TrimSuffix(strings.TrimPrefix(m.edit.Command().Args[2], "true '"), "'")
	original.QuotedLines[0] = "+mutated"
	if got := m.editAnchor.QuotedLines[0]; got != "+return true" {
		t.Fatalf("captured anchor changed: %q", got)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	msg := editFinished{body: " \n", anchor: m.editAnchor, fromPatch: true, generation: m.editGeneration, reservation: m.editReservation}
	cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("empty composition did not signal completion")
	}
	if got, ok := cmd().(Cancelled); !ok || !got.FromPatch {
		t.Fatalf("completion = %#v", cmd())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestEditorAndStorageFailuresFromPatchAreRouted(t *testing.T) {
	m := New(Initial{}, Dependencies{Save: func(data.Draft) (data.Comment, error) { return data.Comment{}, errors.New("disk full") }})
	t.Setenv("EDITOR", "")
	cmd := m.Begin(patch.Anchor{})
	if got, ok := cmd().(Failed); !ok || got.Err == nil || got.Err.Error() != "$EDITOR is not set" {
		t.Fatalf("editor error event = %#v", cmd())
	}

	t.Setenv("EDITOR", "true")
	cmd = m.Begin(patch.Anchor{})
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	cmd = m.Update(editFinished{body: "feedback", fromPatch: true, generation: m.editGeneration, reservation: m.editReservation})
	if cmd == nil {
		t.Fatal("save callback was not scheduled")
	}
	result := cmd()
	cmd = m.Update(result)
	if got, ok := cmd().(Failed); !ok || got.Err == nil || got.Err.Error() != "disk full" {
		t.Fatalf("save error event = %#v", cmd())
	}
}

func TestSameCommentCannotHaveOverlappingMutation(t *testing.T) {
	called := 0
	m := New(Initial{Items: []data.Comment{{ID: "one", Body: "before"}}}, Dependencies{
		Save: func(draft data.Draft) (data.Comment, error) {
			called++
			return data.Comment{ID: draft.ID, Body: draft.Body}, nil
		},
		Delete: func(string) error { called++; return nil },
	})
	t.Setenv("EDITOR", "true")
	if m.begin("one", patch.Anchor{}, "before", false) == nil {
		t.Fatal("editor was not started")
	}
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if cmd := m.delete("one", false); cmd != nil {
		t.Fatal("same ID delete overlapped active edit")
	}
	cmd := m.Update(editFinished{body: "after", id: "one", generation: m.editGeneration, reservation: m.editReservation})
	second := m.delete("one", false)
	if second != nil {
		t.Fatal("same ID delete overlapped active save")
	}
	result := cmd()
	if called != 1 {
		t.Fatalf("storage operations = %d", called)
	}
	m.Update(result)
	if m.items[0].Body != "after" {
		t.Fatalf("saved item = %#v", m.items[0])
	}
}

func TestEmptyEditedCommentDeletesByCapturedID(t *testing.T) {
	deleted := ""
	m := New(Initial{Items: []data.Comment{{ID: "captured", Body: "before"}}}, Dependencies{Delete: func(id string) error { deleted = id; return nil }})
	t.Setenv("EDITOR", "true")
	m.begin("captured", patch.Anchor{}, "before", false)
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(editFinished{body: "\n", id: "captured", generation: m.editGeneration, reservation: m.editReservation})
	if cmd == nil {
		t.Fatal("delete was not scheduled")
	}
	m.Update(cmd())
	if deleted != "captured" || len(m.items) != 0 {
		t.Fatalf("deleted=%q items=%#v", deleted, m.items)
	}
}

func TestSuggestionFenceAndUnchangedStripping(t *testing.T) {
	anchor := patch.Anchor{QuotedLines: []string{"-old", "+new```x", "+last"}}
	draft := commentDraft("explain", anchor)
	if !strings.Contains(draft, "````suggestion\nnew```x\nlast\n````") {
		t.Fatalf("draft = %q", draft)
	}
	if got := stripUnchangedSuggestion(draft, anchor.QuotedLines); got != "explain" {
		t.Fatalf("stripped = %q", got)
	}
	edited := strings.Replace(draft, "new```x", "better", 1)
	if got := stripUnchangedSuggestion(edited, anchor.QuotedLines); got != edited {
		t.Fatalf("edited suggestion was stripped: %q", got)
	}
}

func TestCommentListRenderingEditingDeletionAndRoutingKeys(t *testing.T) {
	t.Setenv("EDITOR", "true")
	deleted := ""
	m := New(Initial{Width: 80, Height: 8, Items: []data.Comment{{ID: "one", Anchor: patch.Anchor{FilePath: "one.go", NewStart: 4}, Body: "old body"}}}, Dependencies{
		Save: func(draft data.Draft) (data.Comment, error) {
			return data.Comment{ID: draft.ID, Anchor: draft.Anchor, Body: draft.Body}, nil
		},
		Delete: func(id string) error { deleted = id; return nil },
	})
	if rendered := ansi.Strip(m.Render()); !strings.Contains(rendered, "comments  1 pending") || !strings.Contains(rendered, "one.go:4  old body") {
		t.Fatalf("render = %q", rendered)
	}
	if cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})); cmd == nil {
		t.Fatal("edit key did not start editor")
	}
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(editFinished{body: "new body", id: "one", anchor: m.editAnchor, generation: m.editGeneration, reservation: m.editReservation})
	if cmd == nil {
		t.Fatal("edit did not schedule save")
	}
	m.Update(cmd())
	if m.items[0].Body != "new body" {
		t.Fatalf("edited body = %q", m.items[0].Body)
	}
	cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "D", Code: 'D'}))
	if cmd == nil {
		t.Fatal("delete key did not schedule delete")
	}
	m.Update(cmd())
	if deleted != "one" || len(m.items) != 0 {
		t.Fatalf("deleted=%q items=%#v", deleted, m.items)
	}
	if cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'})); cmd == nil {
		t.Fatal("q did not request back navigation")
	}
	if _, ok := cmd().(BackRequested); !ok {
		t.Fatalf("q result = %#v", cmd())
	}
	if cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})); cmd == nil {
		t.Fatal("Ctrl-C did not request quit")
	}
	if _, ok := cmd().(QuitRequested); !ok {
		t.Fatalf("Ctrl-C result = %#v", cmd())
	}
	for _, text := range []string{"C", "esc"} {
		code := rune(0)
		if text == "C" {
			code = 'C'
		}
		cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: text, Code: code}))
		if cmd == nil {
			t.Fatalf("%s did not request back navigation", text)
		}
		if _, ok := cmd().(BackRequested); !ok {
			t.Fatalf("%s result = %#v", text, cmd())
		}
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]data.Comment, 10)
	for index := range items {
		items[index] = data.Comment{Body: fmt.Sprintf("comment %d", index), Anchor: patch.Anchor{FilePath: "main.go"}}
	}
	m := New(Initial{Width: 80, Height: 7, Items: items}, Dependencies{})
	m.row = len(items) - 1
	rendered := strings.Split(ansi.Strip(m.Render()), "\n")
	if !strings.Contains(strings.Join(rendered[1:m.height-2], "\n"), "comment 9") {
		t.Fatalf("selected comment is outside the screen body: %q", rendered)
	}
	if !strings.Contains(rendered[m.height-2], "j/k move") {
		t.Fatalf("footer line = %q", rendered[m.height-2])
	}
}

func TestShowReloadFailureKeepsExistingItems(t *testing.T) {
	m := New(Initial{Items: []data.Comment{{ID: "keep", Body: "keep me"}}}, Dependencies{List: func() ([]data.Comment, error) { return nil, errors.New("storage unavailable") }})
	cmd := m.Update(Show{})
	m.Update(cmd())
	if len(m.items) != 1 || m.items[0].ID != "keep" {
		t.Fatalf("items = %#v", m.items)
	}
	if rendered := ansi.Strip(m.Render()); !strings.Contains(rendered, "refresh comments: storage unavailable") {
		t.Fatalf("render = %q", rendered)
	}
}

func TestSaveAndDeleteFailuresKeepCurrentItem(t *testing.T) {
	m := New(Initial{Items: []data.Comment{{ID: "one", Body: "keep"}}}, Dependencies{
		Save:   func(data.Draft) (data.Comment, error) { return data.Comment{}, errors.New("save failed") },
		Delete: func(string) error { return errors.New("delete failed") },
	})
	t.Setenv("EDITOR", "true")
	m.begin("one", patch.Anchor{}, "keep", false)
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(editFinished{body: "edited", id: "one", generation: m.editGeneration, reservation: m.editReservation})
	m.Update(cmd())
	if len(m.items) != 1 || m.items[0].Body != "keep" || m.err == nil || m.err.Error() != "save failed" {
		t.Fatalf("after save failure: items=%#v err=%v", m.items, m.err)
	}
	cmd = m.delete("one", false)
	m.Update(cmd())
	if len(m.items) != 1 || m.items[0].Body != "keep" || m.err == nil || m.err.Error() != "delete failed" {
		t.Fatalf("after delete failure: items=%#v err=%v", m.items, m.err)
	}
}

func TestPatchOriginatedSaveEmitsSaved(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := New(Initial{}, Dependencies{Save: func(draft data.Draft) (data.Comment, error) { return data.Comment{ID: "saved", Body: draft.Body}, nil }})
	if m.Begin(patch.Anchor{FilePath: "a.go"}) == nil {
		t.Fatal("Begin did not start editor")
	}
	if _, err := m.edit.Finish(nil); err != nil {
		t.Fatal(err)
	}
	cmd := m.Update(editFinished{body: "feedback", anchor: patch.Anchor{FilePath: "a.go"}, fromPatch: true, generation: m.editGeneration, reservation: m.editReservation})
	if cmd == nil {
		t.Fatal("save was not scheduled")
	}
	cmd = m.Update(cmd())
	if cmd == nil {
		t.Fatal("successful Patch save emitted no routing event")
	}
	if event, ok := cmd().(Saved); !ok || !event.FromPatch {
		t.Fatalf("save event = %#v", cmd())
	}
}
