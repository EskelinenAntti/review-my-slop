package app

import (
	"fmt"
	"slices"
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

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{
			"z",
			diffscreen.Center,
		}, {"t", diffscreen.Top}, {"b", diffscreen.Bottom},
	} {
		m := testModel(longModelPatch(), nil, nil)
		m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view", test.key,
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("G"))
	last := lineText(m)
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), last, "%s",
		"pending prefix moved focus",
	)

	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), "package main", "gg line=%q",
		lineText(m),
	)
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 30})
	initial := m.diffView.Render(nil)
	want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil), "%s",
		"right keys did not scroll by two steps",
	)

	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"left keys did not restore initial offset",
	)

	m = updateModel(t, m, textKey("$"))
	testutil.RequireEqual(t, m.diffView.Render(nil) != initial, true, "%s",
		"$ did not scroll",
	)

	m = updateModel(t, m, textKey("0"))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"0 did not restore initial offset",
	)
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.currentPatch.Branch = "main"
	m.refreshPatch = func(kind patch.Kind) (patch.Patch, error) {
		p := coveragePatch()
		p.Kind, p.Branch = kind, "main"
		return p, nil
	}
	next, _ := m.Update(textKey("tab"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, m.kind, patch.Branch, "kind=%v", m.kind)

	stale := coveragePatch()
	stale.Files[0].Metadata = []string{"stale"}
	m = updateModel(t, m, patchRefreshFinishedMsg{patch: stale})
	testutil.RequireEqual(t,
		!slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"stale"},
		), true,
		"%s", "stale refresh applied",
	)

	m = updateModel(t, m, textKey("tab"))
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"kind=%v after toggling back to local", m.kind,
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	refreshed := false
	m.refreshPatch = func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return coveragePatch(), nil
	}
	next, cmd := m.Update(textKey("tab"))
	m = requireModel(t, next)
	testutil.RequireEqual(t, cmd == nil, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, !refreshed, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
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
