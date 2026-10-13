package diffscreen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	rendered string,
	width int,
	expected sgrExpectation,
) {
	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = state.assertStyledSpan(
				t,
				rendered,
				column,
				expected,
			)
			break
		}
		column = state.assertStyledSpan(
			t,
			rendered[:location[0]],
			column,
			expected,
		)
		state.apply(rendered[location[2]:location[3]])
		rendered = rendered[location[1]:]
	}
	testutil.RequireEqual(t, column, width,
		fmt.Sprintf("styled columns = %d, want %d", column, width),
	)
}

func (s sgrState) assertStyledSpan(
	t *testing.T,
	text string,
	column int,
	expected sgrExpectation,
) int {
	t.Helper()
	for range text {
		column++
		assertSGRState(t, column, s, expected)
	}
	return column
}

func (s *sgrState) apply(parameters string) {
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
		case 27:
			s.reverse = false
		case 49:
			s.background = ""
		case 48:
			index = s.applyBackground(values, index)
		default:
			continue
		}
	}
}

func (s *sgrState) applyBackground(values []string, index int) int {
	if index+4 < len(values) && values[index+1] == "2" {
		s.background = strings.Join(values[index+2:index+5], ";")
		return index + 4
	}
	return index
}

func assertSGRState(
	t *testing.T,
	column int,
	state sgrState,
	expected sgrExpectation,
) {
	t.Helper()
	testutil.RequireEqual(t, state.background, expected.background,
		fmt.Sprintf("column %d state=%#v want=%#v", column, state, expected),
	)
	testutil.RequireEqual(t, state.reverse, expected.reverse,
		fmt.Sprintf("column %d state=%#v want=%#v", column, state, expected),
	)
}
