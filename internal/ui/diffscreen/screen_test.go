package diffscreen

import (
	"strings"
	"testing"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && file.NewPath == "first.go" && len(lines) == 1 &&
			lines[0].Text == "before",
		true,
		"default selection=%#v %#v %v",
		file,
		lines,
		ok,
	)
	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 2, true,
		"extended selection=%#v",
		lines,
	)
	before := v.cursor
	for range minimumWidth {
		v.Move(NextLine)
	}
	requireEqual(
		t,
		v.cursor.row >= before.row,
		true,
		fixtureV,
		"selection moved backward",
	)
	file, _, _ = v.Selected()
	requireEqual(
		t,
		file.NewPath, "first.go",
		fixtureV,
		"selection crossed file",
	)
	v.ClearSelection()
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 1, true,
		"cleared selection=%#v",
		lines,
	)
}

func TestUpdateAndConfigurePreserveState(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Resize(wideTestWidth, 10)
	v.Move(NextLine)
	v.ToggleSelection()
	v.Move(NextLine)
	_, want, _ := v.Current()
	p.Files[0].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"updated current=%#v, want %#v",
		got,
		want,
	)
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 2, true,
		"updated selection=%#v",
		lines,
	)
	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"configured current=%#v, want %#v",
		got,
		want,
	)
	requireEqual(
		t,
		strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"split layout not rendered",
	)
	before := v.Render(nil)
	v.Update(p)
	requireEqual(
		t,
		v.Render(nil), before,
		fixtureV,
		"unchanged refresh changed presentation",
	)
}

func TestNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = fixtureLongGo, fixtureLongGo
	v := New(p, Options{SideBySide: true, Dark: true})
	v.Resize(wideTestWidth, 10)
	for range 10 {
		v.Move(NextLine)
	}
	v.Align(Center)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(defaultWidth, 10)
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"narrow layout remained split",
	)
	v.Resize(wideTestWidth, 10)
	_, got, _ := v.Current()
	requireEqual(
		t,
		got == want && v.cursor.row-v.viewport.top == offset,
		true,
		fixtureV,
		"resize lost meaningful position",
	)
	requireEqual(
		t,
		strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"resize lost split preference",
	)
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	v.Resize(defaultWidth, defaultHeight)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	requireEqual(
		t,
		v.cursor == beforeCursor && v.viewport == beforeViewport,
		true,
		fixtureV,
		"rejected selection partially changed state",
	)
}
