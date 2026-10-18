package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	_, line, _ := m.diffView.Current()
	testutil.RequireEqual(t, line.Text != "more()", true, "%s",
		"diffSelection crossed hunk",
	)
}

func updateModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	return requireModel(t, next)
}

func requireModel(t *testing.T, next tea.Model) model {
	t.Helper()
	result, ok := next.(model)
	testutil.RequireEqual(t, ok, true, "model=%T", next)

	return result
}
func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}
func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func focusLine(t *testing.T, m *model, text string) {
	t.Helper()
	m.diffView.Move(diffscreen.FirstLine)
	count := 0
	for _, file := range m.currentPatch.Files {
		for _, hunk := range file.Hunks {
			count += len(hunk.Lines)
		}
	}
	for range count {
		if lineText(*m) == text {
			return
		}
		m.diffView.Move(diffscreen.NextLine)
	}
	t.Fatalf("line %q not found", text)
}

func lineText(m model) string {
	_, line, _ := m.diffView.Current()
	return line.Text
}

func screenBody(rendered string) string {
	lines := strings.Split(rendered, "\n")
	return strings.Join(lines[1:len(lines)-2], "\n")
}

func coveragePatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				OldSource:   "package main\nold()\nkeep()\n",
				NewSource:   "package main\nnew()\nkeep()\nmore()\n",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1,3 +1,3 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "package main",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: 3,
								NewNumber: 3,
							},
						},
					},
					{
						Header: "@@ -3,1 +3,2 @@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "keep()",
								OldNumber: 3,
								NewNumber: 3,
							},
							{
								Kind:      patch.Addition,
								Text:      "more()",
								NewNumber: 4,
							},
						},
					},
				},
			},
		},
	}
}

func longModelPatch() patch.Patch {
	lines := make([]patch.Line, 30)
	for index := range lines {
		lines[index] = patch.Line{
			Kind: patch.Context,
			Text: fmt.Sprintf(
				"line %d %s",
				index,
				strings.Repeat("x", 80),
			),
			OldNumber: patch.LineNumber(index + 1),
			NewNumber: patch.LineNumber(index + 1),
		}
	}
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long.go",
				Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}
