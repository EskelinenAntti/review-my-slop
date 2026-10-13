package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "deletion not found",
	)
	testutil.RequireEqual(t, v.cursor.pane, left,
		"search did not activate deletion pane",
	)

	first := v.cursor
	v.AcceptSearch()
	testutil.RequireEqual(t, v.Find(Forward), true, "next match not found")
	testutil.RequireEqual(t, v.cursor != first, true, "next match not found")
	testutil.RequireEqual(t, v.Find(Backward), true,
		"previous match not found",
	)
	testutil.RequireEqual(t, v.cursor, first, "previous match not found")

	v.BeginSearch()
	v.InsertSearch("missing")
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "no matches"),
		true, "missing query matched",
	)

	v.CancelSearch()
	testutil.RequireEqual(t, v.cursor, first, "cancel did not restore focus")

	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	testutil.RequireEqual(t, v.cursor, first,
		"empty query did not restore focus",
	)

	v.CancelSearch()
	v.Move(FirstLine)
	testutil.RequireEqual(t, v.cursor, origin, "first line changed")
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
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("restored source=%#v, want %#v", got, origin),
	)
	testutil.RequireEqual(t, got, origin,
		fmt.Sprintf("restored source=%#v, want %#v", got, origin),
	)
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "/removed界"),
		true, "query chunks were not appended",
	)

	v.BackspaceSearch()
	_, found, ok := v.Current()
	testutil.RequireEqual(t, ok, true,
		"Unicode backspace did not restore a matching query",
	)
	testutil.RequireEqual(t, strings.Contains(found.Text, "removed"), true,
		"Unicode backspace did not restore a matching query",
	)
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "Unicode backspace did not restore a matching query",
	)

	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "/removed"), true,
		"reconfiguration lost the active query",
	)

	v.CancelSearch()
	_, restored, _ := v.Current()
	testutil.RequireEqual(t, restored, origin,
		"cancel did not restore source focus",
	)

	v.BeginSearch()
	v.BackspaceSearch()
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "/removed"),
		true, "new search retained the previous query",
	)
}
