package ui

import (
	"strings"

	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func patchStripANSI(value string) string {
	var result strings.Builder
	for len(value) > 0 {
		start := strings.Index(value, "\x1b[")
		if start < 0 {
			_, _ = result.WriteString(value)
			break
		}
		_, _ = result.WriteString(value[:start])
		end := strings.IndexByte(value[start:], 'm')
		if end < 0 {
			_, _ = result.WriteString(value[start:])
			break
		}
		value = value[start+end+1:]
	}
	return result.String()
}

func newDiffscreenTestView(patch patches.Patch,
	options Options) *Diff {
	view := &Diff{
		view:    &diffView{},
		options: options,
		width:   80,
		height:  30,
	}
	view.Update(patch)
	view.viewport = diffViewport{}
	view.selection = nil
	view.drag = nil
	view.search = searchState{}
	return view
}

func diffscreenTestViewWithTheme(p patches.Patch, dark bool) *diffView {
	return buildDiffView(p, Options{Dark: dark})
}

func diffscreenTestSplitViewWithTheme(p patches.Patch,
	dark bool) *diffView {
	return buildDiffView(p, Options{
		SideBySide: true,
		Dark:       dark,
	})
}

func diffscreenRenderAtCursor(
	v *diffView, target, active diffCursor, width int,
	selection *diffSelection) string {
	viewport := func() diffViewport {
		var viewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, width), max(1, 1)
		{
			var viewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(
					v.rows,
				) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(
				0,
				min(viewport.LeftColumn, contentWidth(v, &viewport)),
			)
			return viewport
		}
	}()
	viewport.top = target.row
	return renderPatch(v, viewport, active, selection)
}

func diffscreenLongContextPatch() patches.Patch {
	lines := make([]patches.Line, 20)
	for index := range lines {
		lines[index] = patches.Line{
			Kind:      patches.Context,
			Text:      strings.Repeat("long", 20),
			OldNumber: patches.LineNumber(index + 1),
			NewNumber: patches.LineNumber(index + 1),
		}
	}
	return patches.Patch{
		Files: []patches.File{
			{
				DisplayPath: "long.go",
				Hunks:       []patches.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}

func diffscreenTwoFileSelectionPatch() patches.Patch {
	return patches.Patch{Root: "/repo", Files: []patches.File{
		{
			DisplayPath: "first.go",
			OldPath:     "first.go",
			NewPath:     "first.go",
			Hunks: []patches.Hunk{{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patches.Line{
					{
						Kind:      patches.Context,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patches.Deletion,
						Text:      "removed one",
						OldNumber: 2,
					},
					{
						Kind:      patches.Deletion,
						Text:      "removed two",
						OldNumber: 3,
					},
					{Kind: patches.Addition, Text: "added one", NewNumber: 2},
					{
						Kind:      patches.Context,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				},
			}},
		},
		{
			DisplayPath: "second.go",
			OldPath:     "second.go",
			NewPath:     "second.go",
			Hunks: []patches.Hunk{
				{
					Header: "@@ -1 +1 @@",
					Lines: []patches.Line{
						{Kind: patches.Addition, Text: "other", NewNumber: 1},
					},
				},
			},
		},
	}}
}

func diffscreenSearchCandidateCursor(v *diffView,

) (diffCursor, bool) {
	return firstCursor(v)
}

func diffscreenSelectionLastLine(selection diffSelection,
	v *diffView) (patches.Line, bool) {
	_, line, ok := patchLineAtCursor(v, selection.Last)
	return line, ok
}

func diffscreenSelectionFirstLine(selection diffSelection,
	v *diffView) (patches.Line, bool) {
	_, line, ok := patchLineAtCursor(v, selection.First)
	return line, ok
}
