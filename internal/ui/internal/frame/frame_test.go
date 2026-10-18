package frame

import (
	"slices"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderPadsAndTruncatesBody(t *testing.T) {
	for _, tt := range []struct {
		name string
		body []string
		want []string
	}{
		{"nil", nil, []string{"", "", ""}},
		{"empty", []string{}, []string{"", "", ""}},
		{"short", []string{"one"}, []string{"one", "", ""}},
		{"exact", []string{"one", "two", "three"},
			[]string{"one", "two", "three"}},
		{"long", []string{"one", "two", "three", "four"},
			[]string{"one", "two", "three"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := slices.Clone(tt.body)
			got := Render("header", tt.body, "footer", 6)
			want := "header\n" + strings.Join(tt.want, "\n") + "\nfooter\n"
			testutil.RequireEqual(t, got, want,
				"Render = %q, want %q", got, want,
			)
			testutil.RequireEqual(t, slices.Equal(tt.body, body), true,
				"Render changed body to %#v", tt.body,
			)
		})
	}
}

func TestRenderReservesAtLeastOneBodyRow(t *testing.T) {
	for _, height := range []int{-1, 0, 1, 2, 3, 4} {
		got := Render("header", []string{"one", "two"}, "footer", height)
		if want := "header\none\nfooter\n"; got != want {
			t.Fatalf("Render at height %d = %q, want %q", height, got, want)
		}
	}
}
