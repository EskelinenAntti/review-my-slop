package app

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type patchRefreshMsg struct {
	patch patch.Patch
	kind  patch.Kind
	err   error
}

type commentEditorFinishedMsg struct {
	body string
	err  error
}

type commentsLoadedMsg struct {
	comments []comments.Comment
	revision uint64
	err      error
}

type sourceEditorFinishedMsg struct{ err error }
