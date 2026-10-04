package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	if !ok || file.NewPath != "first.go" || len(lines) != 1 || lines[0].Text != "before" {
		t.Fatalf("default selection=%#v %#v %v", file, lines, ok)
	}
	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	if !ok || len(lines) != 2 {
		t.Fatalf("extended selection=%#v", lines)
	}
	before := v.cursor
	for range 20 {
		v.Move(NextLine)
	}
	if v.cursor.row < before.row {
		t.Fatal("selection moved backward")
	}
	file, _, _ = v.Selected()
	if file.NewPath != "first.go" {
		t.Fatal("selection crossed file")
	}
	v.ClearSelection()
	_, lines, ok = v.Selected()
	if !ok || len(lines) != 1 {
		t.Fatalf("cleared selection=%#v", lines)
	}
}

func TestUpdateAndConfigurePreserveState(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Resize(120, 10)
	v.Move(NextLine)
	v.ToggleSelection()
	v.Move(NextLine)
	_, want, _ := v.Current()
	p.Files[0].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	if !ok || got != want {
		t.Fatalf("updated current=%#v, want %#v", got, want)
	}
	_, lines, ok := v.Selected()
	if !ok || len(lines) != 2 {
		t.Fatalf("updated selection=%#v", lines)
	}
	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	if !ok || got != want {
		t.Fatalf("configured current=%#v, want %#v", got, want)
	}
	if !strings.Contains(v.Render(nil), " │ ") {
		t.Fatal("split layout not rendered")
	}
	before := v.Render(nil)
	v.Update(p)
	if v.Render(nil) != before {
		t.Fatal("unchanged refresh changed presentation")
	}
}

func TestNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	v := New(p, Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	for range 10 {
		v.Move(NextLine)
	}
	v.Align(Center)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(80, 10)
	if strings.Contains(v.Render(nil), " │ ") {
		t.Fatal("narrow layout remained split")
	}
	v.Resize(120, 10)
	_, got, _ := v.Current()
	if got != want || v.cursor.row-v.viewport.top != offset {
		t.Fatal("resize lost meaningful position")
	}
	if !strings.Contains(v.Render(nil), " │ ") {
		t.Fatal("resize lost split preference")
	}
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	if v.cursor != beforeCursor || v.viewport != beforeViewport {
		t.Fatal("rejected selection partially changed state")
	}
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	if strings.Contains(v.Render(nil), "no matches") {
		t.Fatal("deletion not found")
	}
	if v.cursor.pane != left {
		t.Fatal("search did not activate deletion pane")
	}
	first := v.cursor
	v.AcceptSearch()
	if !v.Find(Forward) || v.cursor == first {
		t.Fatal("next match not found")
	}
	if !v.Find(Backward) || v.cursor != first {
		t.Fatal("previous match not found")
	}
	v.BeginSearch()
	v.InsertSearch("missing")
	if !strings.Contains(v.Render(nil), "no matches") {
		t.Fatal("missing query matched")
	}
	v.CancelSearch()
	if v.cursor != first {
		t.Fatal("cancel did not restore focus")
	}
	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	if v.cursor != first {
		t.Fatal("empty query did not restore focus")
	}
	v.CancelSearch()
	v.Move(FirstLine)
	if v.cursor != origin {
		t.Fatal("first line changed")
	}
}

func TestSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Move(NextLine)
	_, origin, _ := v.Current()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	if !ok || got != origin {
		t.Fatalf("restored source=%#v, want %#v", got, origin)
	}
}

func TestEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patch.Patch{
		{},
		{Files: []patch.File{{DisplayPath: "binary", Metadata: []string{"Binary files differ"}}}},
	} {
		v := New(p, Options{})
		v.Resize(10, 1)
		for motion := PreviousLine; motion <= OtherPane; motion++ {
			v.Move(motion)
		}
		v.Align(Center)
		v.ToggleSelection()
		v.ScrollHorizontal(100)
		v.BeginSearch()
		v.InsertSearch("anything")
		v.CancelSearch()
		if _, _, ok := v.Current(); ok {
			t.Fatal("empty view returned focused code")
		}
		if _, _, ok := v.Selected(); ok {
			t.Fatal("empty view returned selected code")
		}
		if v.Find(Forward) {
			t.Fatal("empty view found a match")
		}
		_ = v.Render(nil)
	}
}

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := New(longPatch(), Options{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	if before == 0 {
		t.Fatal("fixture has no horizontal overflow")
	}
	v.ScrollHorizontal(distance)
	if v.viewport.LeftColumn != before {
		t.Fatal("scroll end overflowed")
	}
	v.ScrollHorizontal(-distance)
	if v.viewport.LeftColumn != 0 {
		t.Fatal("scroll start not clamped")
	}
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	if !strings.Contains(v.Render(nil), "/removed界") {
		t.Fatal("query chunks were not appended")
	}
	v.BackspaceSearch()
	_, found, ok := v.Current()
	if !ok || !strings.Contains(found.Text, "removed") || strings.Contains(v.Render(nil), "no matches") {
		t.Fatal("Unicode backspace did not restore a matching query")
	}
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	if !strings.Contains(v.Render(nil), "/removed") {
		t.Fatal("reconfiguration lost the active query")
	}
	v.CancelSearch()
	_, restored, _ := v.Current()
	if restored != origin {
		t.Fatal("cancel did not restore source focus")
	}
	v.BeginSearch()
	v.BackspaceSearch()
	if strings.Contains(v.Render(nil), "/removed") {
		t.Fatal("new search retained the previous query")
	}
}
