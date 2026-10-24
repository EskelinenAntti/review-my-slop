package patch

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/sourcegraph/go-diff/diff"
)

func TestParseTrackedFilePreservesPathsMetadataAndHunks(t *testing.T) {
	parsed := &diff.FileDiff{
		OrigName: "a/old\nname.go",
		NewName:  "b/new\nname.go",
		Extended: []string{
			"rename from old\nname.go",
			"rename to new\nname.go",
		},
		Hunks: []*diff.Hunk{{
			OrigStartLine: 10,
			OrigLines:     2,
			NewStartLine:  20,
			NewLines:      2,
			Section:       "func value()",
			Body:          []byte(" same\n-old\x1b\n+new\tvalue\n"),
		}},
	}
	got, err := parseTrackedFile(parsed)
	testutil.NoError(t, err)
	want := File{
		OldPath:     "old\nname.go",
		NewPath:     "new\nname.go",
		DisplayPath: `new\nname.go`,
		Metadata: []string{
			`rename from old\nname.go`,
			`rename to new\nname.go`,
		},
		Hunks: []Hunk{{
			Header: "@@ -10,2 +20,2 @@ func value()",
			Lines: []Line{
				{Kind: Context, Text: "same", OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, Text: `old\x1b`, OldNumber: 11},
				{Kind: Addition, Text: "new\tvalue", NewNumber: 21},
			},
		}},
	}
	testutil.RequireEqual(t, reflect.DeepEqual(got, want), true,
		"file = %#v, want %#v", got, want,
	)
}

func TestParseTrackedFiles(t *testing.T) {
	raw := "diff --git a/added.txt b/added.txt\n" +
		"new file mode 100644\n--- /dev/null\n+++ b/added.txt\n" +
		"@@ -0,0 +1 @@\n+added\n" +
		"diff --git a/deleted.txt b/deleted.txt\n" +
		"deleted file mode 100644\n--- a/deleted.txt\n+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n-deleted\n"
	files, err := parseTracked(context.Background(), []byte(raw))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(files), 2, "files = %#v", files)
	for index, want := range []struct {
		oldPath, newPath, displayPath string
		line                          Line
	}{
		{"", "added.txt", "added.txt", Line{
			Kind: Addition, Text: "added", NewNumber: 1,
		}},
		{"deleted.txt", "", "deleted.txt", Line{
			Kind: Deletion, Text: "deleted", OldNumber: 1,
		}},
	} {
		file := files[index]
		testutil.RequireEqual(t, file.OldPath, want.oldPath, "file = %#v", file)
		testutil.RequireEqual(t, file.NewPath, want.newPath, "file = %#v", file)
		testutil.RequireEqual(t, file.DisplayPath, want.displayPath,
			"file = %#v", file,
		)
		testutil.RequireEqual(t, len(file.Hunks), 1, "file = %#v", file)
		testutil.RequireEqual(t,
			reflect.DeepEqual(file.Hunks[0].Lines, []Line{want.line}), true,
			"hunk = %#v", file.Hunks[0],
		)
		testutil.RequireEqual(t, file.OldSource, "", "file = %#v", file)
		testutil.RequireEqual(t, file.NewSource, "", "file = %#v", file)
	}
}

func TestParseTrackedEmptyInput(t *testing.T) {
	for _, raw := range []string{"", " \t\n"} {
		files, err := parseTracked(context.Background(), []byte(raw))
		testutil.NoError(t, err)
		testutil.RequireEqual(t, files == nil, true, "files = %#v", files)
	}
}

func TestParseTrackedRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	raw := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	files, err := parseTracked(ctx, []byte(raw))
	testutil.RequireEqual(t, errors.Is(err, context.Canceled), true,
		"error = %v", err,
	)
	testutil.RequireEqual(t, files == nil, true, "files = %#v", files)
}

func TestParseTrackedFileRejectsMalformedHunk(t *testing.T) {
	file, err := parseTrackedFile(&diff.FileDiff{
		OrigName: "a/file.txt",
		NewName:  "b/file.txt",
		Hunks:    []*diff.Hunk{{Body: []byte("?invalid\n")}},
	})
	testutil.RequireEqual(t, err != nil, true, "error = %v", err)
	testutil.RequireEqual(t, err.Error(),
		"file.txt: unexpected diff prefix '?'", "error = %v", err,
	)
	testutil.RequireEqual(t, reflect.DeepEqual(file, File{}), true,
		"file = %#v", file,
	)
}

func TestParseHunkBodyEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name             string
		oldLine, newLine int32
		body             string
		want             []Line
	}{
		{
			name: "empty body",
			want: []Line{},
		},
		{
			name: "blank source lines", oldLine: 10, newLine: 20,
			body: " \n-\n+\n \n",
			want: []Line{
				{Kind: Context, OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, OldNumber: 11},
				{Kind: Addition, NewNumber: 21},
				{Kind: Context, OldNumber: 12, NewNumber: 22},
			},
		},
		{
			name: "addition-only hunk", newLine: 1,
			body: "+first\n+second",
			want: []Line{
				{Kind: Addition, Text: "first", NewNumber: 1},
				{Kind: Addition, Text: "second", NewNumber: 2},
			},
		},
		{
			name: "deletion-only hunk", oldLine: 1,
			body: "-first\n-second\n",
			want: []Line{
				{Kind: Deletion, Text: "first", OldNumber: 1},
				{Kind: Deletion, Text: "second", OldNumber: 2},
			},
		},
		{
			name:    "newline markers do not advance counters",
			oldLine: 10, newLine: 20,
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same\n",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
		},
		{
			name: "marker-only body",
			body: "\\ No newline at end of file\n",
			want: []Line{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := parseHunkBody(tc.oldLine, tc.newLine, []byte(tc.body))
			testutil.NoError(t, err)
			testutil.RequireEqual(t, reflect.DeepEqual(lines, tc.want), true,
				"lines = %#v, want %#v", lines, tc.want,
			)
		})
	}
}

func TestParseHunkBodyRejectsMalformedLines(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantError string
	}{
		{"missing prefix", "\n", "malformed empty diff line"},
		{
			"empty line after valid line", "+valid\n\n",
			"malformed empty diff line",
		},
		{
			"invalid prefix after valid line", "+valid\n?invalid\n",
			"unexpected diff prefix '?'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := parseHunkBody(1, 1, []byte(tc.body))
			testutil.RequireEqual(t, err != nil, true,
				"lines = %#v, error = %v", lines, err,
			)
			testutil.RequireEqual(t, err.Error(), tc.wantError,
				"error = %v", err,
			)
			testutil.RequireEqual(t, lines == nil, true,
				"malformed input returned partial lines: %#v", lines,
			)
		})
	}
}
