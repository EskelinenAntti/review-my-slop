package ui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

type fakeEditor struct {
	bodies []string
	opened []string
}

func (f *fakeEditor) EditComment(string, comment.Anchor) (tea.Cmd, error) {
	body := ""
	if len(f.bodies) > 0 {
		body = strings.Clone(f.bodies[0])
		f.bodies = f.bodies[1:]
	}
	return func() tea.Msg { return ui.CommentEditedMsg{Body: body} }, nil
}

func (f *fakeEditor) OpenSource(path string, line int) (tea.Cmd, error) {
	f.opened = append(f.opened, fmt.Sprintf("%s:%d", path, line))
	return func() tea.Msg { return ui.SourceEditedMsg{} }, nil
}

func newTestModel(editor ui.Editor, dependencies ui.Dependencies, comments ...comment.Comment) ui.Model {
	dependencies.Editor = editor
	return ui.New(sampleChanges(), comments, dependencies, ui.Layout{Size: ui.Size{Width: 100, Height: 20}})
}

func sendKey(t *testing.T, model ui.Model, key string) ui.Model {
	t.Helper()
	return sendMessage(t, model, keyMessage(key))
}

func sendText(t *testing.T, model ui.Model, text string) ui.Model {
	t.Helper()
	for _, char := range text {
		model = sendMessage(t, model, keyMessage(string(char)))
	}
	return model
}

func sendKeyCommand(t *testing.T, model *ui.Model, key string) tea.Cmd {
	t.Helper()
	next, command := model.Update(keyMessage(key))
	*model = next.(ui.Model)
	return command
}

func finishCommand(t *testing.T, model *ui.Model, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatal("expected command")
	}
	next, _ := model.Update(command())
	*model = next.(ui.Model)
}

func sendMessage(t *testing.T, model ui.Model, message tea.Msg) ui.Model {
	t.Helper()
	next, _ := model.Update(message)
	return next.(ui.Model)
}

func keyMessage(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func equalQuoted(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sampleChanges() diff.ChangeSet {
	return diff.ChangeSet{
		Repository:  "/repo",
		Fingerprint: "fingerprint",
		Files: []diff.File{{
			OldPath:     "main.go",
			NewPath:     "main.go",
			DisplayPath: "main.go",
			OldSource:   "package main\nold()\nkeep()\n",
			NewSource:   "package main\nnew()\nkeep()\nmore()\n",
			Hunks: []diff.Hunk{
				{Header: "@@ -1,3 +1,3 @@", Lines: []diff.Line{{Kind: diff.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: diff.Deletion, Text: "old()", OldNumber: 2}, {Kind: diff.Addition, Text: "new()", NewNumber: 2}, {Kind: diff.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}},
				{Header: "@@ -3,1 +3,2 @@", Lines: []diff.Line{{Kind: diff.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: diff.Addition, Text: "more()", NewNumber: 4}}},
			},
		}},
	}
}
