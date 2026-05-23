package main

import (
	"strings"
	"testing"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

func TestFlowTitleAndBranchNameUseDescriptionTitle(t *testing.T) {
	title := prTitle("# Automatic branch name\n\nDetails here.")
	if title != "Automatic branch name" {
		t.Fatalf("prTitle = %q, want %q", title, "Automatic branch name")
	}

	got := branchName(title)
	want := "automatic-branch-name"
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
