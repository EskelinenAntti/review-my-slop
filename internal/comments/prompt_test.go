package comments

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	testutil.NoError(t, WritePrompt(&out, []Comment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		testutil.RequireEqual(t, strings.Contains(out.String(), expected),
			true, fmt.Sprintf("output lacks %q:\n%s", expected, out.String()),
		)
	}
	testutil.RequireEqual(t, !strings.Contains(out.String(), "batch"), true,
		fmt.Sprintf("output exposes internal batches:\n%s", out.String()),
	)
}

func TestWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	comments := []Comment{
		testComment("/repo", "First."),
		testComment("/repo", "Second."),
	}
	testutil.NoError(t, WritePrompt(&out, comments))

	for _, expected := range []string{"### 1.", "### 2."} {
		testutil.RequireEqual(t, strings.Contains(out.String(), expected),
			true, fmt.Sprintf("output lacks %q:\n%s", expected, out.String()),
		)
	}
}
