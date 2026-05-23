package main

import (
	"strings"
	"testing"
	"time"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

func TestFlowBranchNameUsesPromptTitle(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 11, 12, 0, time.UTC)
	got := branchName("# Add Codex review loop!\n\nDetails here.", now)
	want := "rms/add-codex-review-loop-20260523-101112"
	if got != want {
		t.Fatalf("branchName = %q, want %q", got, want)
	}
}

func TestReviewFixPromptIncludesThreads(t *testing.T) {
	got := reviewFixPrompt([]github.Thread{{
		Path: "main.go",
		Line: 42,
		Comments: []github.ThreadComment{{
			Author: "alice",
			Body:   "Please handle this edge case.",
		}},
	}})
	if !strings.Contains(got, "main.go:42") || !strings.Contains(got, "alice: Please handle this edge case.") {
		t.Fatalf("reviewFixPrompt missing thread context:\n%s", got)
	}
}
