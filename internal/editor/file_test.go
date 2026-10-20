package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentFileRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		quoted []string
	}{
		{name: "empty"},
		{name: "without quote", body: "comment"},
		{
			name: "unchanged suggestion", body: "comment",
			quoted: []string{"-old", "+new"},
		},
		{name: "deletion only", body: "comment", quoted: []string{"-old"}},
		{name: "embedded fence", body: "comment", quoted: []string{"+````"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor := comments.Anchor{QuotedLines: tc.quoted}
			path, err := createCommentFile(tc.body, anchor)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(path) })
			body, err := readCommentFile(path, anchor)
			if err != nil || body != tc.body {
				t.Fatalf("body = %q, err = %v; want %q", body, err, tc.body)
			}
		})
	}
}

func TestReadCommentFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")
	body, err := readCommentFile(path, comments.Anchor{})
	if body != "" || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if !strings.HasPrefix(err.Error(), "read comment file: ") {
		t.Fatalf("missing read context: %v", err)
	}
}

func TestReadCommentFilePreservesEditedSuggestion(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old", "+new"}}
	path, err := createCommentFile("comment", anchor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	edited := "comment\n\n```suggestion\nbetter\n```\n"
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := readCommentFile(path, anchor)
	if err != nil || body != edited {
		t.Fatalf("body = %q, err = %v; want %q", body, err, edited)
	}
}
