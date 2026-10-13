package diffscreen

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type rowKind uint8

const (
	fileRow rowKind = iota
	metadataRow
	hunkRow
	lineRow
)

type displayRow struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type patchProjection struct {
	patch patch.Patch
	rows  []displayRow
	split bool
	dark  bool
}

func newPatchProjection(p patch.Patch, options Options) *patchProjection {
	v := &patchProjection{
		patch: p,
		split: options.SideBySide,
		dark:  options.Dark,
	}
	for fileIndex, file := range p.Files {
		v.appendFileRows(fileIndex, file)
	}
	return v
}

func (v *patchProjection) appendFileRows(fileIndex int, file patch.File) {
	v.rows = append(
		v.rows,
		displayRow{
			kind:      fileRow,
			file:      fileIndex,
			hunk:      -1,
			leftLine:  -1,
			rightLine: -1,
			text:      file.DisplayPath,
		},
	)
	for _, metadata := range file.Metadata {
		v.rows = append(
			v.rows,
			displayRow{
				kind:      metadataRow,
				file:      fileIndex,
				hunk:      -1,
				leftLine:  -1,
				rightLine: -1,
				text:      metadata,
			},
		)
	}
	highlighted := v.highlightFile(file)
	for hunkIndex, hunk := range file.Hunks {
		v.appendHunkRows(fileIndex, hunkIndex, hunk, highlighted)
	}
}

func (v *patchProjection) appendHunkRows(
	fileIndex, hunkIndex int,
	hunk patch.Hunk,
	highlighted highlightedSources,
) {
	v.rows = append(
		v.rows,
		displayRow{
			kind:      hunkRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  -1,
			rightLine: -1,
			text:      hunkHeader(hunk.Header),
		},
	)
	for _, indices := range v.projectHunkRows(hunk.Lines) {
		row := displayRow{
			kind:      lineRow,
			file:      fileIndex,
			hunk:      hunkIndex,
			leftLine:  indices[0],
			rightLine: indices[1],
		}
		v.attachHighlightedSources(&row, hunk.Lines, highlighted)
		v.rows = append(v.rows, row)
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}
