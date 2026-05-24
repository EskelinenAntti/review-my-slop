package slop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

func (s *reviewState) reviewComment(term *terminalState) error {
	return s.reviewWithBody(term, "")
}

func (s *reviewState) reviewSuggestion(term *terminalState) error {
	if !s.hasChangedLines() {
		return errors.New("No changed line selected.")
	}
	if err := s.requireReviewableSource("add suggestion"); err != nil {
		return err
	}
	if err := s.requirePR("build suggestion"); err != nil {
		return err
	}
	if err := s.requireDraft("add suggestion"); err != nil {
		return err
	}
	reviewRange, err := s.currentRange()
	if err != nil {
		return err
	}
	if reviewRange.End.Side != "new" {
		return errors.New("Suggestions are only available on the right side.")
	}
	template, err := suggestionTemplate(reviewRange, s.pr)
	if err != nil {
		return err
	}
	return s.reviewWithBody(term, template)
}

func (s *reviewState) reviewWithBody(term *terminalState, template string) error {
	if !s.hasChangedLines() {
		return errors.New("No changed line selected.")
	}
	if err := s.requireReviewableSource("add comment"); err != nil {
		return err
	}
	if err := s.requirePR("post review comments"); err != nil {
		return err
	}
	if err := s.requireDraft("add comment"); err != nil {
		return err
	}
	reviewRange, err := s.currentRange()
	if err != nil {
		return err
	}
	body, err := editReviewBody(term, template)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		s.message = "Cancelled empty review body."
		return nil
	}
	if err := github.AddPendingReviewComment(s.draft.ID, githubRange(reviewRange), body); err != nil {
		return err
	}
	s.draft.Count++
	s.clearSelection()
	if reviewRange.Start.Line == reviewRange.End.Line {
		s.message = fmt.Sprintf("Added draft comment %d on %s:%d.", s.draft.Count, reviewRange.End.File, reviewRange.End.Line)
	} else {
		s.message = fmt.Sprintf("Added draft comment %d on %s:%d-%d.", s.draft.Count, reviewRange.End.File, reviewRange.Start.Line, reviewRange.End.Line)
	}
	return nil
}

func (s *reviewState) startReview() {
	if err := s.requireReviewableSource("start review"); err != nil {
		s.message = err.Error()
		return
	}
	if err := s.requirePR("start review"); err != nil {
		s.message = err.Error()
		return
	}
	if s.draft.Active {
		s.message = fmt.Sprintf("Review already active with %d draft %s.", s.draft.Count, plural(s.draft.Count, "comment", "comments"))
		return
	}
	reviewID, err := github.CreatePendingReview(s.pr)
	if err != nil {
		s.message = err.Error()
		return
	}
	s.draft = reviewDraft{Active: true, ID: reviewID}
	s.clearSelection()
	s.message = "Draft review started."
}

func (s *reviewState) discardReview() {
	if err := s.requireReviewableSource("delete draft review"); err != nil {
		s.message = err.Error()
		return
	}
	if !s.draft.Active {
		s.message = "No draft review to delete."
		return
	}
	count := s.draft.Count
	if err := github.DeletePendingReview(s.draft.ID); err != nil {
		s.message = err.Error()
		return
	}
	s.draft = reviewDraft{}
	s.clearSelection()
	s.message = fmt.Sprintf("Deleted draft review with %d %s.", count, plural(count, "comment", "comments"))
}

func (s *reviewState) submitReview(term *terminalState, event github.ReviewEvent) error {
	if err := s.requireReviewableSource("submit review"); err != nil {
		return err
	}
	if err := s.requirePR("submit review"); err != nil {
		return err
	}
	if !s.canSubmitReviewEvent(event) {
		return ownPullRequestReviewError(event)
	}
	if !s.draft.Active {
		return errors.New("No draft review active. Press P to start one.")
	}

	body, err := editReviewBody(term, "")
	if err != nil {
		return err
	}

	count := s.draft.Count
	if err := github.SubmitPendingReview(s.draft.ID, body, event); err != nil {
		return err
	}
	s.draft = reviewDraft{}
	s.clearSelection()
	s.message = fmt.Sprintf("Submitted %s review with %d %s.", reviewEventLabel(event), count, plural(count, "comment", "comments"))
	return nil
}

func (s *reviewState) canSubmitReviewEvent(event github.ReviewEvent) bool {
	if event == github.ReviewComment {
		return true
	}
	return !s.ownPullRequest()
}

func ownPullRequestReviewError(event github.ReviewEvent) error {
	switch event {
	case github.ReviewApprove:
		return errors.New("Cannot approve your own pull request.")
	case github.ReviewRequestChanges:
		return errors.New("Cannot request changes on your own pull request.")
	default:
		return nil
	}
}

func (s *reviewState) ownPullRequest() bool {
	return s.pr != nil && s.pr.Author != "" && s.pr.Viewer != "" && strings.EqualFold(s.pr.Author, s.pr.Viewer)
}

func reviewEventLabel(event github.ReviewEvent) string {
	switch event {
	case github.ReviewApprove:
		return "approval"
	case github.ReviewRequestChanges:
		return "request-changes"
	default:
		return "comment"
	}
}

func (s *reviewState) openPR(term *terminalState) {
	if err := s.requirePR("PR"); err != nil {
		s.message = err.Error()
		return
	}
	if err := withNormalTerminal(term, func() error {
		return github.OpenPR()
	}); err != nil {
		s.message = err.Error()
		return
	}
	s.message = fmt.Sprintf("Opened PR #%d.", s.pr.Number)
}

func (s *reviewState) requirePR(action string) error {
	if s.pr != nil {
		return nil
	}
	if s.prChecking {
		return fmt.Errorf("Checking for an active GitHub PR. Cannot %s yet.", action)
	}
	return fmt.Errorf("No active GitHub PR found. Cannot %s.", action)
}

func (s *reviewState) requireDraft(action string) error {
	if s.draft.Active {
		return nil
	}
	return fmt.Errorf("No draft review active. Cannot %s.", action)
}

func (s *reviewState) canReviewBranchChanges() bool {
	return s.reviewable || s.source == sourceBranch
}

func (s *reviewState) requireReviewableSource(action string) error {
	if s.canReviewBranchChanges() {
		return nil
	}
	return fmt.Errorf("Review actions are only available while reviewing branch changes. Cannot %s.", action)
}

func editReviewBody(term *terminalState, template string) (string, error) {
	file, err := os.CreateTemp("", "slop-review-*.md")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.WriteString(template); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	if err := withNormalTerminal(term, func() error {
		return openEditorFile(path)
	}); err != nil {
		return "", err
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func suggestionTemplate(reviewRange reviewRange, pr *prContext) (string, error) {
	if pr == nil {
		return "", errors.New("No active GitHub PR found. Cannot build suggestion.")
	}
	lines, err := sourceLines(reviewRange, pr)
	if err != nil {
		return "", err
	}
	content := strings.Join(lines, "\n")
	fence := suggestionFence(content)
	return fence + "suggestion\n" + content + "\n" + fence + "\n", nil
}

func suggestionFence(content string) string {
	longest := 0
	current := 0
	for _, ch := range content {
		if ch == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return strings.Repeat("`", max(3, longest+1))
}

func sourceLines(reviewRange reviewRange, pr *prContext) ([]string, error) {
	ref := pr.Head
	if reviewRange.End.Side == "old" {
		ref = pr.Base
	}
	out, err := gitShowFile(ref, reviewRange.End.File)
	if err != nil {
		return nil, err
	}
	lines := splitSourceLines(out)
	if reviewRange.Start.Line < 1 || reviewRange.End.Line > len(lines) {
		return nil, fmt.Errorf("cannot read %s:%d-%d from %s", reviewRange.End.File, reviewRange.Start.Line, reviewRange.End.Line, ref)
	}
	return lines[reviewRange.Start.Line-1 : reviewRange.End.Line], nil
}

func githubRange(reviewRange reviewRange) github.LineRange {
	return github.LineRange{
		Start: github.Line{
			File: reviewRange.Start.File,
			Line: reviewRange.Start.Line,
			Side: reviewRange.Start.Side,
		},
		End: github.Line{
			File: reviewRange.End.File,
			Line: reviewRange.End.Line,
			Side: reviewRange.End.Side,
		},
	}
}

func gitShowFile(ref, path string) (string, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("git show failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func splitSourceLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}
