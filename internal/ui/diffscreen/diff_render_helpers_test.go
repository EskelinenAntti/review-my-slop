package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func renderOne(
	v *diffView,
	cursor diffCursor,
	width int,
	selection *diffSelection,
) string {
	return v.renderTarget(cursor, cursor, width, selection)
}

func (v *diffView) renderTarget(
	target, active diffCursor,
	width int,
	selection *diffSelection,
) string {
	viewport := v.newViewport(width, 1)
	viewport.top = target.row
	return v.render(viewport, active, selection)
}

func longLinePatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "long",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind: patch.Context,
								Text: "abcdefghijklmnopqrstuvwxyz" +
									"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
}
