package diffscreen

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"testing"
)

func TestUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := cursorDeletionFixtures()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(
				fmt.Sprintf("%s/split=%v", tc.name, split),
				func(t *testing.T) {
					v := New(original, Options{SideBySide: split})
					v.Resize(120, 4)
					v.BeginSearch()
					v.InsertSearch("target")
					v.AcceptSearch()
					_, line, ok := v.Current()
					testutil.RequireEqual(t, ok, true,
						fmt.Sprintf("initial cursor = %#v, ok=%v", line, ok),
					)
					testutil.RequireEqual(t, line, target,
						fmt.Sprintf("initial cursor = %#v, ok=%v", line, ok),
					)

					v.Update(patchWithHunks(tc.hunks))
					_, line, ok = v.Current()
					testutil.RequireEqual(t, ok, true,
						fmt.Sprintf("restored cursor = %#v, ok=%v, want %#v",
							line, ok, tc.want,
						),
					)
					testutil.RequireEqual(t, line, tc.want,
						fmt.Sprintf("restored cursor = %#v, ok=%v, want %#v",
							line, ok, tc.want,
						),
					)
					assertCursorVisible(t, v)
				},
			)
		}
	}
}

func ptr[T any](value T) *T { return &value }

type cursorDeletionCase struct {
	name  string
	hunks []patch.Hunk
	want  patch.Line
}

func cursorDeletionFixtures() (
	original patch.Patch,
	target patch.Line,
	cases []cursorDeletionCase,
) {
	early := patch.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "early change", NewNumber: 1},
	}}
	before := patch.Line{
		Kind:      patch.Context,
		Text:      "before",
		OldNumber: 50,
		NewNumber: 50,
	}
	added := patch.Line{
		Kind:      patch.Addition,
		Text:      "removed addition",
		NewNumber: 51,
	}
	after := patch.Line{
		Kind:      patch.Context,
		Text:      "after",
		OldNumber: 51,
		NewNumber: 52,
	}
	target = patch.Line{Kind: patch.Addition, Text: "target", NewNumber: 53}
	late := patch.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "late change", NewNumber: 100},
	}}
	original = patchWithHunks([]patch.Hunk{early, {
		Header: "@@ -50,2 +50,4 @@", Lines: []patch.Line{
			before,
			added,
			after,
			target,
		},
	}, late})
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--
	cases = []cursorDeletionCase{
		{"line before cursor deleted", []patch.Hunk{early, {
			Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{
				before,
				shiftedAfter,
				shiftedTarget,
			},
		}, late}, shiftedTarget},
		{"cursor line deleted", []patch.Hunk{early, {
			Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{
				before,
				added,
				after,
			},
		}, late}, after},
		{"cursor hunk removed", []patch.Hunk{early, late}, late.Lines[0]},
	}
	return original, target, cases
}

func assertCursorVisible(t *testing.T, v *View) {
	t.Helper()
	top := v.viewport.top
	bottom := top + v.view.contentHeight(v.viewport)
	testutil.RequireEqual(t, v.cursor.row >= top, true,
		fmt.Sprintf("cursor row %d is above viewport %d:%d", v.cursor.row, top,
			bottom,
		),
	)
	testutil.RequireEqual(t, v.cursor.row < bottom, true,
		fmt.Sprintf("cursor row %d is below viewport %d:%d", v.cursor.row, top,
			bottom,
		),
	)
}
