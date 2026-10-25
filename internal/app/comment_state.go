package app

import (
	"slices"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

// commentState invalidates outstanding loads whenever a local mutation changes
// the comments. Applying a matching load does not invalidate other loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentsLoadedMsg struct {
	comments []comments.Comment
	revision uint64
	err      error
}

func (s *commentState) upsert(comment comments.Comment, index int) {
	if index >= 0 {
		s.items[index] = comment
	} else {
		s.items = append(s.items, comment)
	}
	s.revision++
}

func (s *commentState) remove(index int) {
	s.items = slices.Delete(s.items, index, index+1)
	s.revision++
}

func (s *commentState) applyLoad(msg commentsLoadedMsg) (bool, error) {
	if msg.revision != s.revision {
		return false, nil
	}
	if msg.err != nil {
		return false, msg.err
	}
	s.items = msg.comments
	return true, nil
}
