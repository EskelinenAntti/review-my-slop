package commentscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderErrorKeepsScreenLayout(t *testing.T) {
	v := New([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(
		ansi.Strip(
			v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60))),
		),
		"\n",
	)
	testutil.RequireEqual(t, len(failed), len(normal),
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"),
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.HasPrefix(failed[5], "storage failed "),
		true, fmt.Sprintf("error footer=%q", failed[5]),
	)
	testutil.RequireEqual(t, ansi.StringWidth(failed[5]), 25,
		fmt.Sprintf("error footer=%q", failed[5]),
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "j/k move"), true,
		fmt.Sprintf("error footer=%q", failed[5]),
	)
}
