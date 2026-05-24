package github

import (
	"strings"
	"testing"
)

func TestGraphQLErrorMessageExtractsGitHubErrors(t *testing.T) {
	out := []byte(`{"data":{"submitPullRequestReview":null},"errors":[{"type":"UNPROCESSABLE","path":["submitPullRequestReview"],"locations":[{"line":3,"column":3}],"message":"Could not resolve to a PullRequestReview with the id \"review-id\""}]}`)

	got, ok := graphQLErrorMessage(out)
	if !ok {
		t.Fatal("expected GraphQL error message")
	}
	for _, want := range []string{
		"submitPullRequestReview",
		"Could not resolve",
		"review-id",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, `"data"`) || strings.Contains(got, `"errors"`) {
		t.Fatalf("message included raw GraphQL envelope: %q", got)
	}
}

func TestGraphQLErrorMessageExtractsErrorsBeforeGHFooter(t *testing.T) {
	out := []byte(`{"data":{"submitPullRequestReview":null},"errors":[{"type":"UNPROCESSABLE","path":["submitPullRequestReview"],"locations":[{"line":3,"column":3}],"message":"Could not comment for pull request review. You need to leave a comment indicating the requested changes."}]}gh: Could not comment for pull request review. You need to leave a comment indicating
the requested changes.`)

	got, ok := graphQLErrorMessage(out)
	if !ok {
		t.Fatal("expected GraphQL error message")
	}
	for _, want := range []string{
		"submitPullRequestReview",
		"Could not comment for pull request review",
		"You need to leave a comment",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, `"data"`) || strings.Contains(got, "gh:") {
		t.Fatalf("message included raw command output: %q", got)
	}
}
