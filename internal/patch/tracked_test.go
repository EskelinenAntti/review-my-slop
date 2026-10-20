package patch

import (
	"reflect"
	"testing"

	"github.com/sourcegraph/go-diff/diff"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseTrackedFile(t *testing.T) {
	tests := []struct {
		name, oldPath, newPath string
		wantOld, wantNew       string
	}{
		{"modified", "a/code.go", "b/code.go", "code.go", "code.go"},
		{"renamed", "a/old.go", "b/new.go", "old.go", "new.go"},
		{"added", "/dev/null", "b/new.go", "", "new.go"},
		{"deleted", "a/old.go", "/dev/null", "old.go", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &diff.FileDiff{
				OrigName: tt.oldPath,
				NewName:  tt.newPath,
				Extended: []string{"metadata\x1b[2J"},
				Hunks: []*diff.Hunk{{
					OrigStartLine: 10, OrigLines: 1,
					NewStartLine: 20, NewLines: 1,
					Section: "function",
					Body:    []byte("-old\n+new\n"),
				}},
			}
			got, err := parseTrackedFile(fd)
			testutil.NoError(t, err)
			want := File{
				OldPath: tt.wantOld, NewPath: tt.wantNew,
				DisplayPath: tt.wantNew,
				Metadata:    []string{visibleText("metadata\x1b[2J")},
				Hunks: []Hunk{{
					Header: "@@ -10,1 +20,1 @@ function",
					Lines: []Line{
						{Kind: Deletion, Text: "old", OldNumber: 10},
						{Kind: Addition, Text: "new", NewNumber: 20},
					},
				}},
			}
			if want.DisplayPath == "" {
				want.DisplayPath = tt.wantOld
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("file = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseTrackedFileWithoutHunks(t *testing.T) {
	got, err := parseTrackedFile(&diff.FileDiff{
		OrigName: "a/old.go", NewName: "b/new.go",
		Extended: []string{"similarity index 100%"},
	})
	testutil.NoError(t, err)
	want := File{
		OldPath: "old.go", NewPath: "new.go", DisplayPath: "new.go",
		Metadata: []string{"similarity index 100%"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file = %#v, want %#v", got, want)
	}
}

func TestParseTrackedFileRejectsMalformedHunk(t *testing.T) {
	got, err := parseTrackedFile(&diff.FileDiff{
		OrigName: "a/code.go", NewName: "b/code.go",
		Hunks: []*diff.Hunk{{Body: []byte("?invalid\n")}},
	})
	if err == nil || err.Error() != "code.go: unexpected diff prefix '?'" {
		t.Fatalf("error = %v, want file-scoped malformed hunk error", err)
	}
	if !reflect.DeepEqual(got, File{}) {
		t.Fatalf("file = %#v, want zero value on error", got)
	}
}

func TestParseHunkBodyBoundaries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []Line
	}{
		{name: "empty body", want: []Line{}},
		{
			name: "marker only",
			body: "\\ No newline at end of file\n",
			want: []Line{},
		},
		{
			name: "no trailing newline",
			body: "+added",
			want: []Line{{Kind: Addition, Text: "added", NewNumber: 20}},
		},
		{
			name: "empty code lines",
			body: " \n-\n+\n",
			want: []Line{
				{Kind: Context, OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, OldNumber: 11},
				{Kind: Addition, NewNumber: 21},
			},
		},
		{
			name: "markers do not advance line numbers",
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same\n",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			testutil.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("lines = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseHunkBodyRejectsMalformedLines(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty line", body: "\n", want: "malformed empty diff line"},
		{
			name: "unknown prefix",
			body: "?code\n",
			want: "unexpected diff prefix '?'",
		},
		{
			name: "malformed line after valid line",
			body: "+valid\n\n",
			want: "malformed empty diff line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			testutil.RequireEqual(t, err != nil, true,
				"error = nil, want %q", tt.want)
			testutil.RequireEqual(t, err.Error(), tt.want,
				"error = %v, want %q", err, tt.want)
			testutil.RequireEqual(t, got == nil, true,
				"lines = %#v, want nil on error", got)
		})
	}
}
