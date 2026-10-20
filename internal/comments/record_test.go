package comments

import (
	"reflect"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDecodeCommentFormats(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		anchor Anchor
		body   string
	}{
		{
			name: "current",
			fields: `"anchor":{"file":"main.go","old_start":1,"old_end":2,` +
				`"new_start":3,"new_end":4,"quoted_lines":["+line"]},` +
				`"body":"new"`,
			anchor: Anchor{
				FilePath: "main.go", OldStart: 1, OldEnd: 2,
				NewStart: 3, NewEnd: 4, QuotedLines: []string{"+line"},
			},
			body: "new",
		},
		{
			name: "legacy",
			fields: `"comment":{"anchor":{"file":"old.go","new_start":3},` +
				`"body":"old"}`,
			anchor: Anchor{FilePath: "old.go", NewStart: 3},
			body:   "old",
		},
		{
			name: "legacy fields take precedence even when empty",
			fields: `"anchor":{"file":"main.go"},"body":"new",` +
				`"comment":{}`,
		},
		{
			name:   "null legacy preserves current fields",
			fields: `"anchor":{"file":"main.go"},"body":"new","comment":null`,
			anchor: Anchor{FilePath: "main.go"},
			body:   "new",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`{"id":"stored","repository":"/repo",` +
				`"created_at":"1970-01-01T00:00:01Z",` + tc.fields + `}`)
			got, err := decodeComment(data)
			testutil.NoError(t, err)
			want := Comment{
				ID: "stored", Repository: "/repo",
				CreatedAt: time.Unix(1, 0).UTC(),
				Anchor:    tc.anchor, Body: tc.body,
			}
			testutil.RequireEqual(t, reflect.DeepEqual(got, want), true,
				"decoded comment = %#v, want %#v", got, want)
		})
	}
}
