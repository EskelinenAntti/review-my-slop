package ui

import (
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func newTestCommentScreen(items []feedback.Comment) *Comments {
	v := &Comments{width: 80, height: 30}
	v.Update(items)
	return v
}
