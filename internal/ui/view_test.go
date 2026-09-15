package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/diff"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func TestUnifiedViewNavigatesChangedLinesAndRejectsCrossHunkSelection(t *testing.T) {
	view := ui.NewUnifiedView(sampleChanges(), true)
	first, ok := view.First()
	if !ok {
		t.Fatal("view has no changed lines")
	}
	line, ok := view.Line(first)
	if !ok || line.Text != "package main" {
		t.Fatalf("first line = %#v", line)
	}
	selection := view.BeginSelection(first)
	for range 10 {
		next, moved := view.Move(first, ui.Forward)
		if !moved {
			break
		}
		first = next
	}
	if _, ok := view.ExtendSelection(selection, first); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestSideBySideViewMapsBothSidesToOneAnchor(t *testing.T) {
	view := ui.NewSideBySideView(sampleChanges(), true)
	cursor, ok := view.First()
	if !ok {
		t.Fatal("view has no changed lines")
	}
	if cursor, ok = view.SwitchPane(cursor, ui.Left); !ok {
		t.Fatal("left pane unavailable at first line")
	}
	for {
		line, lineOK := view.Line(cursor)
		if lineOK && line.Text == "old()" {
			break
		}
		next, moved := view.Move(cursor, ui.Forward)
		if !moved {
			t.Fatal("old line not found")
		}
		cursor = next
	}
	left, ok := view.SwitchPane(cursor, ui.Left)
	if !ok {
		t.Fatal("left pane unavailable")
	}
	right, ok := view.SwitchPane(left, ui.Right)
	if !ok {
		t.Fatal("right pane unavailable")
	}
	selection := view.BeginSelection(left)
	selection, ok = view.ExtendSelection(selection, right)
	if !ok {
		t.Fatal("cross-pane selection rejected")
	}
	anchor, err := view.Anchor(selection)
	if err != nil {
		t.Fatal(err)
	}
	if anchor.FilePath != "main.go" || !strings.Contains(strings.Join(anchor.QuotedLines, "\n"), "old()") || !strings.Contains(strings.Join(anchor.QuotedLines, "\n"), "new()") {
		t.Fatalf("anchor = %#v", anchor)
	}
}

func TestViewSearchesTextAndRendersBothModes(t *testing.T) {
	for _, constructor := range []func(diff.ChangeSet, bool) ui.View{ui.NewUnifiedView, ui.NewSideBySideView} {
		view := constructor(sampleChanges(), true)
		first, _ := view.First()
		found, ok := view.Search("new()", first, ui.Forward)
		if !ok {
			t.Fatal("search did not find changed line")
		}
		line, _ := view.Line(found)
		if line.Text != "new()" {
			t.Fatalf("search line = %#v", line)
		}
		rendered := ansi.Strip(view.Render(view.NewViewport(100, 10), found, nil))
		if !strings.Contains(rendered, "new()") {
			t.Fatalf("rendered view omitted match: %q", rendered)
		}
	}
}

func TestViewReturnsMeaningfulErrorForEmptySelection(t *testing.T) {
	view := ui.NewUnifiedView(diff.ChangeSet{}, true)
	if _, err := view.Anchor(commentSelection()); err == nil {
		t.Fatal("empty selection was accepted")
	}
}

func commentSelection() ui.Selection {
	return ui.Selection{First: ui.Cursor{}, Last: ui.Cursor{}}
}
