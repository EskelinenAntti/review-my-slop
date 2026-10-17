package main

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

func patchHighlightSource(source, filename, theme string) []string {
	if source == "" {
		return nil
	}
	var output bytes.Buffer
	err := quick.Highlight(
		&output,
		source,
		filename,
		"terminal16m",
		theme,
	)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&output)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}

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

func newDiffscreenTestView(patch patchPatch,
	options diffscreenOptions) *diffscreenView {
	view := &diffscreenView{
		view:    &diffscreenDiffView{},
		options: options,
		width:   80,
		height:  30,
	}
	view.Update(patch)
	view.viewport = diffscreenDiffViewport{}
	view.selection = nil
	view.drag = nil
	view.search = diffscreenSearchState{}
	return view
}

func diffscreenTestViewWithTheme(p patchPatch, dark bool) *diffscreenDiffView {
	return diffscreenBuildDiffView(p, diffscreenOptions{Dark: dark})
}

func diffscreenTestSplitViewWithTheme(p patchPatch,
	dark bool) *diffscreenDiffView {
	return diffscreenBuildDiffView(p, diffscreenOptions{
		SideBySide: true,
		Dark:       dark,
	})
}

func diffscreenRenderAtCursor(
	v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
	selection *diffscreenDiffSelection) string {
	viewport := func() diffscreenDiffViewport {
		var viewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, width), max(1, 1)
		{
			var viewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(
					v.rows,
				) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(
				0,
				min(viewport.LeftColumn, diffscreenContentWidth(v, &viewport)),
			)
			return viewport
		}
	}()
	viewport.top = target.row
	return diffscreenRenderPatch(v, viewport, active, selection)
}

func diffscreenLineAtCursor(v *diffscreenDiffView,
	cursor diffscreenDiffCursor) (patchLine, bool) {
	if !(cursor.row >= 0 && cursor.row < len(
		v.rows,
	) && diffscreenLineNumberAtCursor(cursor, v) >= 0) {
		return patchLine{}, false
	}
	current := v.rows[cursor.row]
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
		if current.kind != diffscreenLineRow {
			return -1
		}
		if !v.split || cursor.pane == diffscreenRight {
			return current.rightLine
		}
		return current.leftLine
	}()], true
}

func diffscreenTestSelectionLines(view *diffscreenDiffView,
	selection diffscreenDiffSelection) []patchLine {
	return diffscreenLinesInSelection(view, selection)
}

func diffscreenLongContextPatch() patchPatch {
	lines := make([]patchLine, 20)
	for index := range lines {
		lines[index] = patchLine{
			Kind:      patchContext,
			Text:      strings.Repeat("long", 20),
			OldNumber: patchLineNumber(index + 1),
			NewNumber: patchLineNumber(index + 1),
		}
	}
	return patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "long.go",
				Hunks:       []patchHunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}

func diffscreenTwoFileSelectionPatch() patchPatch {
	return patchPatch{Root: "/repo", Files: []patchFile{
		{
			DisplayPath: "first.go",
			OldPath:     "first.go",
			NewPath:     "first.go",
			Hunks: []patchHunk{{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patchDeletion,
						Text:      "removed one",
						OldNumber: 2,
					},
					{
						Kind:      patchDeletion,
						Text:      "removed two",
						OldNumber: 3,
					},
					{Kind: patchAddition, Text: "added one", NewNumber: 2},
					{
						Kind:      patchContext,
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
			Hunks: []patchHunk{
				{
					Header: "@@ -1 +1 @@",
					Lines: []patchLine{
						{Kind: patchAddition, Text: "other", NewNumber: 1},
					},
				},
			},
		},
	}}
}

func diffscreenSearchCandidateCursor(v *diffscreenDiffView,

) (diffscreenDiffCursor, bool) {
	return diffscreenFirstCursor(v)
}

func diffscreenViewCursorViewport(next *diffscreenDiffView,
	screen *diffscreenView) diffscreenDiffViewport {
	return diffscreenCursorViewport(next, screen.viewport, screen.cursor)
}

func diffscreenSelectionLastLine(selection diffscreenDiffSelection,
	v *diffscreenDiffView) (patchLine, bool) {
	_, line, ok := diffscreenPatchLineAtCursor(v, selection.Last)
	return line, ok
}

func diffscreenSelectionFirstLine(selection diffscreenDiffSelection,
	v *diffscreenDiffView) (patchLine, bool) {
	_, line, ok := diffscreenPatchLineAtCursor(v, selection.First)
	return line, ok
}

func diffscreenSelectedPatchLines(selection *diffscreenDiffSelection,
	v *diffscreenDiffView) []patchLine {
	value := *selection
	if _, ok := diffscreenSelectionFile(v, value); !ok {
		return nil
	}
	return diffscreenLinesInSelection(v, value)
}
