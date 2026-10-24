package comments

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDecodeCommentFormats(t *testing.T) {
	const identity = `"id":"review-1","repository":"/repo",
		"created_at":"1970-01-01T00:00:01Z"`
	const current = `"anchor":{"file":"current.go","old_start":2,"old_end":3,
		"new_start":4,"new_end":5,"quoted_lines":["-old","+new"]},
		"body":"current"`
	const legacy = `"comment":{"anchor":{"file":"legacy.go","new_start":6,
		"quoted_lines":["+legacy"]},"body":"legacy"}`
	currentAnchor := Anchor{
		FilePath: "current.go", OldStart: 2, OldEnd: 3, NewStart: 4, NewEnd: 5,
		QuotedLines: []string{"-old", "+new"},
	}
	legacyAnchor := Anchor{
		FilePath: "legacy.go", NewStart: 6, QuotedLines: []string{"+legacy"},
	}
	for _, tt := range []struct {
		name, fields, body string
		anchor             Anchor
	}{
		{"current", current, "current", currentAnchor},
		{"legacy", legacy, "legacy", legacyAnchor},
		{
			"legacy takes precedence", current + "," + legacy,
			"legacy", legacyAnchor,
		},
		{"null legacy", current + `,"comment":null`, "current", currentAnchor},
		{
			"empty legacy takes precedence", current + `,"comment":{}`,
			"", Anchor{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("{" + identity + "," + tt.fields + "}")
			got, err := decodeComment(data)
			testutil.NoError(t, err)
			want := Comment{
				ID: "review-1", Repository: "/repo",
				CreatedAt: time.Unix(1, 0).UTC(),
				Anchor:    tt.anchor, Body: tt.body,
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("comment = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCommentEncodingRoundTrip(t *testing.T) {
	want := testComment("/repo", "feedback")
	want.Anchor = Anchor{
		FilePath: "main.go", OldStart: 2, OldEnd: 3, NewStart: 4, NewEnd: 5,
		QuotedLines: []string{"-old", "+new"},
	}
	data, err := encodeComment(want)
	testutil.NoError(t, err)
	got, err := decodeComment(data)
	testutil.NoError(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("comment = %#v, want %#v", got, want)
	}
}

func TestDecodeCommentRejectsMalformedRecords(t *testing.T) {
	for _, data := range []string{
		`{`,
		`{"created_at":"invalid"}`,
		`{"anchor":{"new_start":"invalid"}}`,
		`{"comment":{"body":42}}`,
	} {
		t.Run(data, func(t *testing.T) {
			_, err := decodeComment([]byte(data))
			if err == nil ||
				!strings.HasPrefix(err.Error(), "decode comment: ") {
				t.Fatalf("error = %v, want wrapped decode error", err)
			}
		})
	}
}
