package comments

import (
	"reflect"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentEncodingRoundTrip(t *testing.T) {
	want := Comment{
		ID:         "comment-id",
		Repository: "/repository",
		CreatedAt:  time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC),
		Anchor: Anchor{
			FilePath:    "file.go",
			OldStart:    1,
			OldEnd:      2,
			NewStart:    3,
			NewEnd:      4,
			QuotedLines: []string{"-old", "+new"},
		},
		Body: "review feedback",
	}
	data, err := encodeComment(want)
	testutil.NoError(t, err)
	got, err := decodeComment(data)
	testutil.NoError(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded comment = %#v, want %#v", got, want)
	}
}

func TestDecodeCommentLegacyOverridesModernContent(t *testing.T) {
	got, err := decodeComment([]byte(`{
		"id": "comment-id",
		"repository": "/repository",
		"created_at": "2026-10-18T12:00:00Z",
		"anchor": {"file": "modern.go"},
		"body": "modern feedback",
		"comment": {
			"anchor": {"file": "legacy.go", "quoted_lines": ["+old"]},
			"body": "legacy feedback"
		}
	}`))
	testutil.NoError(t, err)
	want := Comment{
		ID:         "comment-id",
		Repository: "/repository",
		CreatedAt:  time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC),
		Anchor: Anchor{
			FilePath:    "legacy.go",
			QuotedLines: []string{"+old"},
		},
		Body: "legacy feedback",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded comment = %#v, want %#v", got, want)
	}
}

func TestDecodeCommentNullLegacyKeepsModernContent(t *testing.T) {
	got, err := decodeComment([]byte(`{
		"anchor": {"file": "modern.go"},
		"body": "modern feedback",
		"comment": null
	}`))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Anchor.FilePath, "modern.go",
		"anchor = %#v", got.Anchor,
	)
	testutil.RequireEqual(t, got.Body, "modern feedback", "body = %q", got.Body)
}

func TestDecodeCommentRejectsMalformedRecords(t *testing.T) {
	for _, data := range []string{
		`{`,
		`{"created_at": "invalid"}`,
		`{"anchor": "invalid"}`,
		`{"comment": {"body": 123}}`,
	} {
		t.Run(data, func(t *testing.T) {
			got, err := decodeComment([]byte(data))
			testutil.RequireEqual(t, err != nil, true, "expected decode error")
			if !reflect.DeepEqual(got, Comment{}) {
				t.Fatalf("failed decode returned %#v", got)
			}
		})
	}
}
