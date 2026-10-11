package diffscreen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func renderOne(
	v *diffView,
	cursor diffCursor,
	width int,
	selection *diffSelection,
) string {
	return renderTarget(v, cursor, cursor, width, selection)
}

func renderTarget(
	v *diffView,
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
						Header: hunkMarker,

						Lines: []patch.Line{
							{
								Kind: patch.Context,

								Text: "abcdefghijklmnopqrstuvwxyz0123456789" +
									"ABCDEFGHIJKLMNOPQRSTUVWXYZ",

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

type sgrExpectation struct {
	background string
	reverse    bool
}

type sgrState struct {
	background string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func assertStyledThroughColumn(
	t *testing.T,
	initialRendered string,
	width int,
	expected sgrExpectation,
) {
	rendered := initialRendered

	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = assertStyledText(t, rendered, column, state, expected)
			break
		}
		column = assertStyledText(
			t,
			rendered[:location[0]],
			column,
			state,
			expected,
		)
		state.apply(rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	requireEqual(
		t,
		column, width,
		"styled columns = %d, want %d",
		column,
		width,
	)
}

func (s *sgrState) apply(initialParameters string) {
	parameters := initialParameters

	if parameters == "" {
		parameters = "0"
	}
	values := strings.Split(parameters, ";")
	for index := 0; index < len(values); index++ {
		value, _ := strconv.Atoi(values[index])
		switch value {
		case 0:
			*s = sgrState{}
		case 7:
			s.reverse = true
		case sgrReverseOff:
			s.reverse = false
		case sgrDefaultBackground:
			s.background = ""
		case sgrExtendedBackground:
			index = s.applyBackground(values, index)
		default:
			// Other inputs leave the current state unchanged.
		}
	}
}

func assertSGRState(
	t *testing.T,
	column int,
	state sgrState,
	expected sgrExpectation,
) {
	t.Helper()
	requireEqual(
		t,
		state.background == expected.background &&
			state.reverse == expected.reverse,
		true,
		"column %d state=%#v want=%#v",
		column,
		state,
		expected,
	)
}

func (s *sgrState) applyBackground(values []string, index int) int {
	if index+4 < len(values) && values[index+1] == "2" {
		s.background = strings.Join(values[index+2:index+5], ";")
		return index + 4
	}
	return index
}

func assertStyledText(
	t *testing.T,
	text string,
	start int,
	state sgrState,
	expected sgrExpectation,
) int {
	column := start
	for range text {
		column++
		assertSGRState(t, column, state, expected)
	}
	return column
}
