package diffscreen

import (
	"strings"
	"testing"
)

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(wideTestWidth, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"deletion not found",
	)
	requireEqual(
		t,
		v.cursor.pane, left,
		fixtureV,
		"search did not activate deletion pane",
	)
	first := v.cursor
	v.AcceptSearch()
	foundForward := v.Find(Forward)
	requireEqual(
		t,
		foundForward && v.cursor != first,
		true,
		fixtureV,
		"next match not found",
	)
	foundBackward := v.Find(Backward)
	requireEqual(
		t,
		foundBackward && v.cursor == first,
		true,
		fixtureV,
		"previous match not found",
	)
	v.BeginSearch()
	v.InsertSearch("missing")
	requireEqual(
		t,
		strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"missing query matched",
	)
	v.CancelSearch()
	requireEqual(t, v.cursor, first, fixtureV, "cancel did not restore focus")
	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	requireEqual(
		t,
		v.cursor, first,
		fixtureV,
		"empty query did not restore focus",
	)
	v.CancelSearch()
	v.Move(FirstLine)
	requireEqual(t, v.cursor, origin, fixtureV, "first line changed")
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
	v.Resize(wideTestWidth, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	requireEqual(
		t,
		ok && got == origin, true,
		"restored source=%#v, want %#v",
		got,
		origin,
	)
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "/removed界"),
		true,
		fixtureV,
		"query chunks were not appended",
	)
	v.BackspaceSearch()
	_, found, ok := v.Current()
	requireEqual(
		t,
		ok && strings.Contains(found.Text, "removed") &&
			!strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"Unicode backspace did not restore a matching query",
	)
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(wideTestWidth, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "/removed"),
		true,
		fixtureV,
		"reconfiguration lost the active query",
	)
	v.CancelSearch()
	_, restored, _ := v.Current()
	requireEqual(
		t,
		restored, origin,
		fixtureV,
		"cancel did not restore source focus",
	)
	v.BeginSearch()
	v.BackspaceSearch()
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), "/removed"),
		true,
		fixtureV,
		"new search retained the previous query",
	)
}
