package comments

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCommentRecordRoundTrip(t *testing.T) {
	want := Comment{
		ID:         "comment-1",
		Repository: "/repository",
		CreatedAt:  time.Date(2026, time.October, 25, 12, 30, 0, 0, time.UTC),
		Anchor: Anchor{
			FilePath:    "main.go",
			OldStart:    2,
			OldEnd:      3,
			NewStart:    4,
			NewEnd:      5,
			QuotedLines: []string{"-old", "+new"},
		},
		Body: "Review feedback",
	}
	data, err := encodeComment(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeComment(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded comment = %#v, want %#v", got, want)
	}
}

func TestDecodeCommentLegacyOverride(t *testing.T) {
	const identity = `"id":"comment-1","repository":"/repository",
		"created_at":"2026-10-25T12:30:00Z"`
	for _, tc := range []struct {
		name, fields string
		want         Comment
	}{
		{
			name: "legacy fields override modern fields",
			fields: `"anchor":{"file":"modern.go"},"body":"modern",
				"comment":{"anchor":{"file":"legacy.go",
				"quoted_lines":["+line"]},"body":"legacy"}`,
			want: Comment{
				Anchor: Anchor{
					FilePath:    "legacy.go",
					QuotedLines: []string{"+line"},
				},
				Body: "legacy",
			},
		},
		{
			name: "empty legacy fields override modern fields",
			fields: `"anchor":{"file":"modern.go"},"body":"modern",
				"comment":{}`,
		},
		{
			name: "null legacy comment retains modern fields",
			fields: `"anchor":{"file":"modern.go"},"body":"modern",
				"comment":null`,
			want: Comment{
				Anchor: Anchor{FilePath: "modern.go"},
				Body:   "modern",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("{" + identity + "," + tc.fields + "}")
			got, err := decodeComment(data)
			if err != nil {
				t.Fatal(err)
			}
			tc.want.ID = "comment-1"
			tc.want.Repository = "/repository"
			tc.want.CreatedAt = time.Date(
				2026, time.October, 25, 12, 30, 0, 0, time.UTC,
			)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decoded comment = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDecodeCommentInvalidRecord(t *testing.T) {
	for _, data := range []string{
		`{`,
		`{"created_at":"invalid"}`,
		`{"body":42}`,
		`{"comment":{"body":42}}`,
	} {
		t.Run(data, func(t *testing.T) {
			_, err := decodeComment([]byte(data))
			if err == nil || !strings.Contains(err.Error(), "decode comment:") {
				t.Fatalf(
					"decodeComment(%q) error = %v, want decode error",
					data, err,
				)
			}
		})
	}
}
