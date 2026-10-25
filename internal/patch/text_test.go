package patch

import (
	"fmt"
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestVisibleTextPreservesPrintableContent(t *testing.T) {
	for _, value := range []string{
		"", "plain text", "日本語 café 😀", `literal\n\x1b`,
	} {
		t.Run(value, func(t *testing.T) {
			testutil.CheckEqual(t, visibleSource(value), value, "source text")
			testutil.CheckEqual(t, visibleText(value), value,
				"single-line text")
		})
	}
}

func TestVisibleTextEscapesControlCharacters(t *testing.T) {
	for control := byte(0); control <= 0x7f; control++ {
		if control >= 0x20 && control != 0x7f {
			continue
		}
		t.Run(fmt.Sprintf("%02x", control), func(t *testing.T) {
			var source, text string
			switch control {
			case '\n':
				source, text = "\n", `\n`
			case '\t':
				source, text = "\t", "\t"
			case '\r':
				source, text = `\r`, `\r`
			default:
				source = fmt.Sprintf(`\x%02x`, control)
				text = source
			}
			value := "before" + string(control) + "after"
			testutil.CheckEqual(t,
				visibleSource(value), "before"+source+"after",
				"source text")
			testutil.CheckEqual(t, visibleText(value), "before"+text+"after",
				"single-line text")
		})
	}
}

func TestVisibleStringsEscapesWithoutChangingInput(t *testing.T) {
	values := []string{"path\nname", "\x1b[31mred", "\tindent\r"}
	original := slices.Clone(values)
	want := []string{`path\nname`, `\x1b[31mred`, "\tindent\\r"}
	got := visibleStrings(values)
	testutil.CheckEqual(t, slices.Equal(got, want), true,
		"visible strings = %q, want %q", got, want)
	testutil.CheckEqual(t, slices.Equal(values, original), true,
		"input changed: %q", values)
}
