package frame

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestErrorRendersSingleLineStatus(t *testing.T) {
	for _, test := range []struct {
		name, value, want string
	}{
		{name: "empty"},
		{name: "single line", value: "storage failed", want: "storage failed"},
		{
			name:  "multiple lines",
			value: "storage failed\ntry again\n",
			want:  "storage failed try again ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Error(test.value)
			testutil.RequireEqual(t, ansi.Strip(got), test.want,
				"error status=%q", got,
			)
			testutil.RequireEqual(t, strings.Contains(got, "\n"), false,
				"error status adds a screen row: %q", got,
			)
			testutil.RequireEqual(t, got, errorStyle.Render(test.want),
				"error status lost its style: %q", got,
			)
		})
	}
}
