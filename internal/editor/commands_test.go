package editor

import (
	"context"
	"os"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	command := newCommentEditorCommand(
		context.Background(), "printf 'edited externally' >", path,
	)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	body, err := readEditedCommentDraft(path, comments.Anchor{})
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}
