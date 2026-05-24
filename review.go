package main

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

func (s *reviewState) submitReview(term *terminalState) error {
	if err := s.requirePR("submit review"); err != nil {
		return err
	}
	if !s.draft.Active {
		return errors.New("No draft review active. Press R to start one.")
	}

	body, err := editReviewBody(term, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" && s.draft.Count == 0 {
		s.message = "Cancelled empty review."
		return nil
	}

	count := s.draft.Count
	if err := github.SubmitPendingReview(s.draft.ID, body); err != nil {
		return err
	}
	s.draft = reviewDraft{}
	s.clearSelection()
	s.message = fmt.Sprintf("Submitted review with %d %s.", count, plural(count, "comment", "comments"))
	return nil
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
	return "```suggestion\n" + strings.Join(lines, "\n") + "\n```\n", nil
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
