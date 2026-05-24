package slop

import "github.com/anttieskelinen/review-my-slop/internal/github"

func detectReviewContextAsync() <-chan reviewContext {
	ch := make(chan reviewContext, 1)
	go func() {
		ch <- detectReviewContext()
	}()
	return ch
}

func detectReviewContext() reviewContext {
	pr := github.DetectPR()
	if pr == nil {
		return reviewContext{}
	}
	return reviewContext{
		PR:    pr,
		Draft: github.DetectPendingReview(pr),
	}
}

func (s *reviewState) receiveReviewContext(ch <-chan reviewContext) bool {
	if !s.prChecking {
		return false
	}
	select {
	case context := <-ch:
		s.applyReviewContext(context)
		return true
	default:
		return false
	}
}

func (s *reviewState) applyReviewContext(context reviewContext) {
	s.prChecking = false
	s.pr = context.PR
	s.draft = context.Draft
	s.branchAvailable = context.PR != nil
}
