package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

const (
	submittedCommentMark = "*"
	pendingCommentMark   = "!"
)

func (s *reviewState) threadForRef(ref lineRef) *github.ReviewThread {
	for i := range s.threads {
		thread := &s.threads[i]
		if thread.File == ref.File && thread.Line == ref.Line && thread.Side == ref.Side {
			return thread
		}
	}
	return nil
}

func (s *reviewState) commentMark(lineIndex int) string {
	for _, changedLine := range s.changedLines {
		if changedLine.LineIndex != lineIndex {
			continue
		}
		for _, ref := range changedLine.refs() {
			if thread := s.threadForRef(ref); thread != nil {
				if thread.Pending {
					return pendingCommentMark
				}
				return submittedCommentMark
			}
		}
	}
	return " "
}

func (s *reviewState) openSelectedComment(term *terminalState) bool {
	if !s.hasChangedLines() {
		return false
	}
	thread := s.threadForRef(s.current())
	if thread == nil {
		thread = s.threadForChangedLine(s.changedLines[s.cursor])
		if thread == nil {
			return false
		}
	}
	if thread.Pending {
		if err := s.editPendingComment(term, thread); err != nil {
			s.message = err.Error()
		}
		return true
	}
	if err := viewReviewThread(term, thread); err != nil {
		s.message = err.Error()
	} else {
		s.message = fmt.Sprintf("Viewed comment thread on %s:%d.", thread.File, thread.Line)
	}
	return true
}

func (s *reviewState) threadForChangedLine(changedLine changedLine) *github.ReviewThread {
	for _, ref := range changedLine.refs() {
		if thread := s.threadForRef(ref); thread != nil {
			return thread
		}
	}
	return nil
}

func (s *reviewState) editPendingComment(term *terminalState, thread *github.ReviewThread) error {
	comment := editablePendingComment(thread)
	if comment == nil {
		return fmt.Errorf("No pending comment found on %s:%d.", thread.File, thread.Line)
	}
	body, err := editReviewBody(term, comment.Body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("Pending comment body cannot be empty.")
	}
	if err := github.UpdatePendingReviewComment(comment.ID, body); err != nil {
		return err
	}
	comment.Body = body
	s.message = fmt.Sprintf("Updated draft comment on %s:%d.", thread.File, thread.Line)
	return nil
}

func editablePendingComment(thread *github.ReviewThread) *github.ReviewComment {
	for i := len(thread.Comments) - 1; i >= 0; i-- {
		if thread.Comments[i].Pending {
			return &thread.Comments[i]
		}
	}
	return nil
}

func viewReviewThread(term *terminalState, thread *github.ReviewThread) error {
	file, err := os.CreateTemp("", "rms-thread-*.md")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.WriteString(formatReviewThread(thread)); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return withNormalTerminal(term, func() error {
		return openEditorFile(path)
	})
}

func formatReviewThread(thread *github.ReviewThread) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s:%d %s\n\n", thread.File, thread.Line, sideLabel(thread.Side))
	for i, comment := range thread.Comments {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		status := ""
		if comment.Pending {
			status = " pending"
		}
		author := comment.Author
		if author == "" {
			author = "unknown"
		}
		fmt.Fprintf(&b, "## %s%s\n\n%s\n", author, status, strings.TrimSpace(comment.Body))
	}
	return b.String()
}

func (s changedLine) refs() []lineRef {
	var refs []lineRef
	if s.Left != nil {
		refs = append(refs, *s.Left)
	}
	if s.Right != nil {
		refs = append(refs, *s.Right)
	}
	if len(refs) == 0 && s.Ref.File != "" {
		refs = append(refs, s.Ref)
	}
	return refs
}
