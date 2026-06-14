package pullrequest

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestOpenChecksOutPullRequestAndLoadsMetadata(t *testing.T) {
	runner := &fakeRunner{responses: [][]byte{
		nil,
		[]byte(`{"id":"PR_node","number":123,"baseRefName":"main"}`),
		[]byte(`{"nameWithOwner":"owner/repo"}`),
	}}
	store, err := Open(context.Background(), "/repo", 123, runner)
	if err != nil {
		t.Fatal(err)
	}
	if store.Number != 123 || store.Base != "main" || store.Repo != "owner/repo" || store.PullRequestID != "PR_node" {
		t.Fatalf("store = %#v", store)
	}
	want := [][]string{
		{"pr", "checkout", "123"},
		{"pr", "view", "123", "--json", "id,number,baseRefName"},
		{"repo", "view", "--json", "nameWithOwner"},
	}
	if got := runner.args(); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestOpenBrowserUsesCurrentOrExplicitPullRequest(t *testing.T) {
	for _, test := range []struct {
		name   string
		number int
		want   []string
	}{
		{name: "current branch", want: []string{"pr", "view", "--web"}},
		{name: "explicit number", number: 123, want: []string{"pr", "view", "123", "--web"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{responses: [][]byte{nil}}
			if err := OpenBrowser(context.Background(), "/repo", test.number, runner); err != nil {
				t.Fatal(err)
			}
			if got := runner.requests[0].args; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("command = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestStoreListsPendingReviewComments(t *testing.T) {
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`[{"id":7,"node_id":"review-node","state":"PENDING"}]`),
		[]byte(`[{"id":9,"node_id":"comment-node","body":"fix this","path":"main.go","diff_hunk":"@@ -1 +1 @@","line":4,"start_line":3,"side":"RIGHT"}]`),
	}}
	store := testStore(runner)
	comments, err := store.List(context.Background(), review.Diff{})
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != "comment-node" || comments[0].Comment.Anchor.NewStart != 3 || comments[0].Comment.Anchor.NewEnd != 4 {
		t.Fatalf("comments = %#v", comments)
	}
}

func TestStoreCreatesPendingReviewForFirstComment(t *testing.T) {
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`[]`),
		[]byte(`{"data":{"addPullRequestReview":{"pullRequestReview":{"id":"review-node","databaseId":7,"comments":{"nodes":[{"id":"comment-node","body":"fix this","path":"main.go","diffHunk":"@@","line":4,"startLine":3}]}}}}}`),
	}}
	store := testStore(runner)
	saved, err := store.Save(context.Background(), newComment(), review.Diff{})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "comment-node" || store.reviewNodeID != "review-node" {
		t.Fatalf("saved = %#v, review node = %q", saved, store.reviewNodeID)
	}
	input := runner.requests[1].inputMap(t)
	variables := input["variables"].(map[string]any)
	fields := variables["input"].(map[string]any)
	if fields["pullRequestId"] != "PR_node" {
		t.Fatalf("GraphQL input = %#v", fields)
	}
	threads := fields["threads"].([]any)
	thread := threads[0].(map[string]any)
	if thread["path"] != "main.go" || thread["line"] != float64(4) || thread["startLine"] != float64(3) || thread["side"] != "RIGHT" {
		t.Fatalf("thread input = %#v", thread)
	}
}

func TestStoreAddsUpdatesAndDeletesDraftComment(t *testing.T) {
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`[{"id":7,"node_id":"review-node","state":"PENDING"}]`),
		[]byte(`{"data":{"addPullRequestReviewThread":{"thread":{"comments":{"nodes":[{"id":"comment-node","body":"fix this","path":"main.go","diffHunk":"@@","line":4,"startLine":3}]}}}}}`),
		[]byte(`{"data":{"updatePullRequestReviewComment":{"pullRequestReviewComment":{"id":"comment-node","body":"edited","path":"main.go","diffHunk":"@@","line":4,"startLine":3}}}}`),
		[]byte(`{"data":{"deletePullRequestReviewComment":{"clientMutationId":null}}}`),
	}}
	store := testStore(runner)
	saved, err := store.Save(context.Background(), newComment(), review.Diff{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Comment.Body = "edited"
	updated, err := store.Save(context.Background(), saved, review.Diff{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != "comment-node" || updated.Comment.Body != "edited" {
		t.Fatalf("updated = %#v", updated)
	}
	if err := store.Delete(context.Background(), updated, review.Diff{}); err != nil {
		t.Fatal(err)
	}
	for index, mutation := range []string{"addPullRequestReviewThread", "updatePullRequestReviewComment", "deletePullRequestReviewComment"} {
		if !strings.Contains(string(runner.requests[index+1].input), mutation) {
			t.Fatalf("request %d does not contain %s: %s", index+1, mutation, runner.requests[index+1].input)
		}
	}
}

func TestStorePreservesLeftSideForGraphQLComments(t *testing.T) {
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`[]`),
		[]byte(`{"data":{"addPullRequestReview":{"pullRequestReview":{"id":"review-node","databaseId":7,"comments":{"nodes":[{"id":"comment-node","body":"fix this","path":"main.go","diffHunk":"@@","line":4,"startLine":3}]}}}}}`),
	}}
	store := testStore(runner)
	comment := newComment()
	comment.Comment.Anchor = review.Anchor{
		File:        "main.go",
		OldStart:    3,
		OldEnd:      4,
		QuotedLines: []string{"-first", "-second"},
	}

	saved, err := store.Save(context.Background(), comment, review.Diff{})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Comment.Anchor.OldStart != 3 || saved.Comment.Anchor.OldEnd != 4 || saved.Comment.Anchor.NewEnd != 0 {
		t.Fatalf("saved anchor = %#v", saved.Comment.Anchor)
	}
	if strings.Contains(string(runner.requests[1].input), "\n\tside\n") {
		t.Fatalf("GraphQL query still requests removed side field: %s", runner.requests[1].input)
	}
}

func TestNewCommentPayloadRejectsSelectionAcrossDiffSides(t *testing.T) {
	comment := newComment().Comment
	comment.Anchor.QuotedLines = []string{"-old", "+new"}
	if _, err := newCommentPayload(comment); err == nil {
		t.Fatal("mixed-side selection was accepted")
	}
}

func newComment() review.StoredComment {
	return review.StoredComment{Comment: review.Comment{
		Body: "fix this",
		Anchor: review.Anchor{
			File:        "main.go",
			NewStart:    3,
			NewEnd:      4,
			QuotedLines: []string{"+first", "+second"},
		},
	}}
}

func testStore(runner Runner) *Store {
	return &Store{
		Runner:        runner,
		Dir:           "/repo",
		Repo:          "owner/repo",
		Number:        123,
		Base:          "main",
		PullRequestID: "PR_node",
	}
}

type fakeRequest struct {
	args  []string
	input []byte
}

func (r fakeRequest) inputMap(t *testing.T) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(r.input, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

type fakeRunner struct {
	requests  []fakeRequest
	responses [][]byte
	err       error
}

func (r *fakeRunner) Run(_ context.Context, _ string, input []byte, args ...string) ([]byte, error) {
	r.requests = append(r.requests, fakeRequest{
		args:  append([]string(nil), args...),
		input: append([]byte(nil), input...),
	})
	if r.err != nil {
		return nil, r.err
	}
	if len(r.responses) == 0 {
		return nil, errors.New("unexpected command")
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response, nil
}

func (r *fakeRunner) args() [][]string {
	result := make([][]string, len(r.requests))
	for index, request := range r.requests {
		result[index] = request.args
	}
	return result
}
