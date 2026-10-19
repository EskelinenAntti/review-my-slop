package comments

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentRecordRoundTrip(t *testing.T) {
	want := testComment("/repo", "feedback")
	want.Anchor = Anchor{
		FilePath: "main.go",
		OldStart: 1, OldEnd: 2,
		NewStart: 3, NewEnd: 4,
		QuotedLines: []string{"-old", "+new"},
	}
	data, err := encodeComment(want)
	testutil.NoError(t, err)
	got, err := decodeComment(data)
	testutil.NoError(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded comment = %#v, want %#v", got, want)
	}
}

func TestDecodeLegacyCommentPrecedence(t *testing.T) {
	for _, legacy := range []struct {
		name, value, body, path string
	}{
		{"absent", "", "current", "current.go"},
		{"null", `,"comment":null`, "current", "current.go"},
		{"populated", `,"comment":{
			"body":"legacy","anchor":{"file":"legacy.go"}
		}`, "legacy", "legacy.go"},
		{"empty", `,"comment":{}`, "", ""},
	} {
		t.Run(legacy.name, func(t *testing.T) {
			data := `{
				"id":"id","repository":"/repo",
				"created_at":"1970-01-01T00:00:01Z",
				"body":"current","anchor":{"file":"current.go"}
			` + legacy.value + `}`
			got, err := decodeComment([]byte(data))
			testutil.NoError(t, err)
			want := testComment("/repo", legacy.body)
			want.ID = "id"
			want.Anchor = Anchor{FilePath: legacy.path}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded comment = %#v, want %#v", got, want)
			}
		})
	}
}

func TestDecodeCommentRejectsInvalidRecords(t *testing.T) {
	for _, data := range []string{
		`{`, `{"created_at":"invalid"}`, `{"body":42}`,
	} {
		_, err := decodeComment([]byte(data))
		if err == nil || !strings.HasPrefix(err.Error(), "decode comment: ") {
			t.Fatalf("decodeComment(%q) error = %v, want decode error",
				data, err,
			)
		}
	}
}
