package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("updated current=%#v, want %#v", got, want),
	)
	testutil.RequireEqual(t, got, want,
		fmt.Sprintf("updated current=%#v, want %#v", got, want),
	)

	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("updated selection=%#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 2,
		fmt.Sprintf("updated selection=%#v", lines),
	)

	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("configured current=%#v, want %#v", got, want),
	)
	testutil.RequireEqual(t, got, want,
		fmt.Sprintf("configured current=%#v, want %#v", got, want),
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"split layout not rendered",
	)

	before := v.Render(nil)
	v.Update(p)
	testutil.RequireEqual(t, v.Render(nil), before,
		"unchanged refresh changed presentation",
	)
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
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), " │ "), true,
		"narrow layout remained split",
	)

	v.Resize(120, 10)
	_, got, _ := v.Current()
	testutil.RequireEqual(t, got, want, "resize lost meaningful position")
	testutil.RequireEqual(t, v.cursor.row-v.viewport.top, offset,
		"resize lost meaningful position",
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"resize lost split preference",
	)
}

func TestEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patch.Patch{
		{},
		{
			Files: []patch.File{
				{
					DisplayPath: "binary",
					Metadata:    []string{"Binary files differ"},
				},
			},
		},
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
		testutil.RequireEqual(t, !v.Find(Forward), true,
			"empty view found a match",
		)

		_ = v.Render(nil)
	}
}
