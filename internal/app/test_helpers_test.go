package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func newTestModel(
	currentPatch patch.Patch,
	initialComments []comments.Comment,
	saveCommentToStore saveCommentToStoreFunc,
) model {
	return newModel(
		currentPatch,
		initialComments,
		saveCommentToStore,
		initialLayout{size: size{Width: 100, Height: 30}},
	)
}

func applyTestMessage(t *testing.T, current model, msg tea.Msg) model {
	t.Helper()
	next, _ := current.Update(msg)
	return requireAppModel(t, next)
}

func requireAppModel(t *testing.T, next tea.Model) model {
	t.Helper()
	result, ok := next.(model)
	testutil.RequireEqual(t, ok, true, fmt.Sprintf("model=%T", next))
	return result
}

func textKeyMessage(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func specialKeyMessage(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func controlKeyMessage(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func focusPatchLine(t *testing.T, current *model, text string) {
	t.Helper()
	current.patchView.Move(diffscreen.FirstLine)
	lineCount := 0
	for _, file := range current.currentPatch.Files {
		for _, hunk := range file.Hunks {
			lineCount += len(hunk.Lines)
		}
	}
	for range lineCount {
		if focusedLineText(*current) == text {
			return
		}
		current.patchView.Move(diffscreen.NextLine)
	}
	t.Fatalf("line %q not found", text)
}

func focusedLineText(current model) string {
	_, line, _ := current.patchView.Current()
	return line.Text
}

func renderedScreenBody(rendered string) string {
	lines := strings.Split(rendered, "\n")
	return strings.Join(lines[1:len(lines)-2], "\n")
}
