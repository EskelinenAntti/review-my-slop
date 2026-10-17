package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func newTestDiffscreenView(p patches.Patch, options ui.Options) *ui.Diff {
	return ui.NewDiff(p, options)
}

func renderTestHelpBindings(
	bindings []ui.Binding,
	width, height int,
) string {
	return ui.RenderHelp(bindings, width, height)
}

func expectedTestHelpBindings() []ui.Binding {
	return []ui.Binding{{
		Keys: "j/k, arrows", Description: "move",
	}, {
		Keys: "h/l, left/right", Description: "scroll horizontally",
	}, {
		Keys: "Mouse wheel", Description: "scroll content",
	}, {
		Keys: "Click/drag", Description: "focus line/select range",
	}, {
		Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane",
	}, {
		Keys: "0/$", Description: "start/end of lines",
	}, {
		Keys: "gg/G", Description: "first/last changed line",
	}, {
		Keys: "zz/zt/zb", Description: "center/top/bottom current line",
	}, {
		Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up",
	}, {
		Keys: "/", Description: "search diff text",
	}, {
		Keys: "n/N", Description: "next/previous search match",
	}, {
		Keys: "]f/[f", Description: "next/previous file",
	}, {
		Keys: "v", Description: "select a line range",
	}, {
		Keys: "c", Description: "comment on selection/current line",
	}, {
		Keys: "e", Description: "open current line in $EDITOR",
	}, {
		Keys: "C", Description: "view comments",
	}, {
		Keys: "R", Description: "refresh diff",
	}, {
		Keys: "Tab", Description: "toggle local/branch changes",
	}, {
		Keys: "t", Description: "toggle unified/side-by-side",
	}, {
		Keys: "q", Description: "quit",
	}}
}

func newTestCommentScreen(items []feedback.Comment) *ui.Comments {
	return ui.NewComments(items)
}

func makeTestPatchWithManyLines() patches.Patch {
	lines := make([]patches.Line, 30)
	for index := range lines {
		lines[index] = patches.Line{
			Kind: patches.Context,
			Text: fmt.Sprintf(
				"line %d %s", index, strings.Repeat("x", 80),
			),
			OldNumber: patches.LineNumber(index + 1),
			NewNumber: patches.LineNumber(index + 1),
		}
	}
	return patches.Patch{Files: []patches.File{{
		DisplayPath: "long.go",
		Hunks:       []patches.Hunk{{Header: "@@", Lines: lines}},
	}}}
}

func newTestBasicAppModel() reviewModel {
	return newTestAppModel(newTestBasicPatch())
}

func newTestAppModel(p patches.Patch) reviewModel {
	layout := initialLayout{size: size{Width: 100, Height: 30}}
	return newTestAppModelWithLayout(p, layout)
}

func newTestAppModelWithLayout(
	p patches.Patch,
	layout initialLayout,
) reviewModel {
	m := createReviewAppModel(
		p, layout.size, nil,
		settingsPreferences{SideBySide: layout.SideBySide}, nil,
	)
	m.save = nil
	m.saveLayout = layout.SaveSideBySide
	return m
}

func updateTestAppModel(
	t *testing.T,
	model reviewModel,
	message tea.Msg,
) reviewModel {
	t.Helper()
	next, _ := model.Update(message)
	result, ok := next.(reviewModel)
	requireEqual(t, ok, true, "model=%T", next)
	return result
}

func appModelFromTestResult(t *testing.T, result tea.Model) reviewModel {
	t.Helper()
	model, ok := result.(reviewModel)
	requireEqual(t, ok, true, "model=%T", result)
	return model
}

func runTestAppCommand(
	t *testing.T, model reviewModel, command tea.Cmd,
) reviewModel {
	t.Helper()
	return updateTestAppModel(t, model, command())
}

func pressTestKey(t *testing.T, model reviewModel, text string) reviewModel {
	t.Helper()
	runes := []rune(text)
	return updateTestAppModel(t, model, tea.KeyPressMsg(tea.Key{
		Text: text,
		Code: runes[0],
	}))
}

func testKeyMessage(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func currentTestDiffLineText(model reviewModel) string {
	_, line, _ := model.diffView.Current()
	return line.Text
}

func testRenderedDiffBody(model reviewModel) string {
	lines := strings.Split(model.diffView.Render(nil), "\n")
	return strings.Join(lines[1:len(lines)-2], "\n")
}

func moveTestDiffToLine(t *testing.T, model *reviewModel, text string) {
	t.Helper()
	model.diffView.Move(ui.FirstLine)
	for range testPatchLineCount(model.currentPatch) {
		if currentTestDiffLineText(*model) == text {
			return
		}
		model.diffView.Move(ui.NextLine)
	}
	t.Fatalf("line %q not found", text)
}

func testPatchLineCount(patch patches.Patch) int {
	count := 0
	for _, file := range patch.Files {
		for _, hunk := range file.Hunks {
			count += len(hunk.Lines)
		}
	}
	return count
}

func newTestBasicPatch() patches.Patch {
	return patches.Patch{Root: "/repo", Files: []patches.File{{
		DisplayPath: "main.go",
		OldPath:     "main.go",
		NewPath:     "main.go",
		OldSource:   "package main\nold()\nkeep()\n",
		NewSource:   "package main\nnew()\nkeep()\nmore()\n",
		Hunks: []patches.Hunk{
			{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patches.Line{
					{
						Kind:      patches.Context,
						Text:      "package main",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patches.Deletion,
						Text:      "old()",
						OldNumber: 2,
					},
					{
						Kind:      patches.Addition,
						Text:      "new()",
						NewNumber: 2,
					},
					{
						Kind:      patches.Context,
						Text:      "keep()",
						OldNumber: 3,
						NewNumber: 3,
					},
				},
			},
			{Header: "@@ -3,1 +3,2 @@", Lines: []patches.Line{{
				Kind:      patches.Context,
				Text:      "keep()",
				OldNumber: 3,
				NewNumber: 3,
			}, {
				Kind:      patches.Addition,
				Text:      "more()",
				NewNumber: 4,
			}}},
		},
	}}}
}

func createTestCommentDraft(anchor feedback.Anchor, body string,

) (string, error) {
	return createCommentDraft(body, anchor)
}

func readTestCommentDraft(anchor feedback.Anchor, path string,

) (string, error) {
	return readCommentDraft(path, anchor)
}

func renderTestAppMode(m reviewModel) string {
	return m.View().Content
}

func newTestEditorCommandAdapter(
	ctx context.Context,
	command, body string,
	anchor feedback.Anchor,
	onComplete func(string, error) tea.Msg,
) (*editorCommandAdapter, tea.ExecCallback) {
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		path, err := createTestCommentDraft(anchor, body)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		if err := runCommentEditor(ctx, command, path, adapter); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readTestCommentDraft(anchor, path)
		return err
	}
	return adapter, func(err error) tea.Msg {
		return onComplete(adapter.body, err)
	}
}

func readTestUIPreferences() (settingsPreferences, error) {
	path, err := userSettingsPath()
	if err != nil {
		return settingsPreferences{}, err
	}
	return readSettingsPreferences(path)
}

func renderAppWorkflowMode(m *reviewModel) string {
	return renderTestAppMode(*m)
}

func updateTestAppWithKey(m *reviewModel, t *testing.T) reviewModel {
	t.Helper()
	return pressTestKey(t, *m, "t")
}

func pressTestAppWithG(m *reviewModel, t *testing.T) reviewModel {
	t.Helper()
	return pressTestKey(t, *m, "g")
}
