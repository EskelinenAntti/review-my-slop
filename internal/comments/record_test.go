package comments

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentRecordRoundTrip(t *testing.T) {
	want := testComment("/repo", "feedback")
	want.Anchor = Anchor{
		FilePath: "main.go", OldStart: 1, OldEnd: 2, NewStart: 3, NewEnd: 4,
		QuotedLines: []string{"-old", "+new"},
	}
	data, err := encodeComment(want)
	testutil.NoError(t, err)
	got, err := decodeComment(data)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, reflect.DeepEqual(got, want), true,
		"decoded = %#v, want %#v", got, want)
}

func TestCommentRecordLegacyFieldsTakePrecedence(t *testing.T) {
	data := []byte(`{
		"id": "comment-id", "repository": "/repo",
		"created_at": "1970-01-01T00:00:01Z",
		"anchor": {"file": "current.go"}, "body": "current",
		"comment": {"anchor": {"file": "legacy.go"}, "body": "legacy"}
	}`)
	got, err := decodeComment(data)
	testutil.NoError(t, err)
	want := testComment("/repo", "legacy")
	want.ID = "comment-id"
	want.Anchor = Anchor{FilePath: "legacy.go"}
	testutil.RequireEqual(t, reflect.DeepEqual(got, want), true,
		"decoded = %#v, want %#v", got, want)
}

func TestCommentRecordRejectsInvalidJSON(t *testing.T) {
	_, err := decodeComment([]byte(`{"body":`))
	if err == nil {
		t.Fatal("invalid JSON was accepted")
	}
}
