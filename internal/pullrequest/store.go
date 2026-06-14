package pullrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

type Runner interface {
	Run(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GH_PAGER=cat")
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
}

type Session struct {
	runner        Runner
	dir           string
	repo          string
	number        int
	baseBranch    string
	pullRequestID string
	reviewID      int64
	reviewNodeID  string
}

type pullRequest struct {
	Number      int    `json:"number"`
	BaseRefName string `json:"baseRefName"`
	ID          string `json:"id"`
}

type repository struct {
	NameWithOwner string `json:"nameWithOwner"`
}

type apiReview struct {
	ID     int64  `json:"id"`
	NodeID string `json:"node_id"`
	State  string `json:"state"`
}

type apiComment struct {
	ID                int64  `json:"id"`
	NodeID            string `json:"node_id"`
	Body              string `json:"body"`
	Path              string `json:"path"`
	DiffHunk          string `json:"diff_hunk"`
	Line              int    `json:"line"`
	StartLine         int    `json:"start_line"`
	Side              string `json:"side"`
	StartSide         string `json:"start_side"`
	OriginalLine      int    `json:"original_line"`
	OriginalStartLine int    `json:"original_start_line"`
}

type graphQLComment struct {
	ID                string `json:"id"`
	Body              string `json:"body"`
	Path              string `json:"path"`
	DiffHunk          string `json:"diffHunk"`
	Line              int    `json:"line"`
	StartLine         int    `json:"startLine"`
	OriginalLine      int    `json:"originalLine"`
	OriginalStartLine int    `json:"originalStartLine"`
}

func Open(ctx context.Context, dir string, number int, runner Runner) (*Session, error) {
	if number <= 0 {
		return nil, errors.New("pull request number must be positive")
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	if _, err := runner.Run(ctx, dir, nil, "pr", "checkout", strconv.Itoa(number)); err != nil {
		return nil, err
	}

	var pr pullRequest
	out, err := runner.Run(ctx, dir, nil, "pr", "view", strconv.Itoa(number), "--json", "id,number,baseRefName")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("decode pull request: %w", err)
	}
	var repo repository
	out, err = runner.Run(ctx, dir, nil, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(out, &repo); err != nil {
		return nil, fmt.Errorf("decode repository: %w", err)
	}
	if pr.Number == 0 || pr.BaseRefName == "" || pr.ID == "" || repo.NameWithOwner == "" {
		return nil, errors.New("gh returned incomplete pull request metadata")
	}
	return &Session{
		runner:        runner,
		dir:           dir,
		repo:          repo.NameWithOwner,
		number:        pr.Number,
		baseBranch:    pr.BaseRefName,
		pullRequestID: pr.ID,
	}, nil
}

func (s *Session) BaseBranch() string {
	return s.baseBranch
}

func (s *Session) OpenBrowser(ctx context.Context) error {
	return OpenBrowser(ctx, s.dir, s.number, s.runner)
}

func OpenBrowser(ctx context.Context, dir string, number int, runner Runner) error {
	if runner == nil {
		runner = ExecRunner{}
	}
	args := []string{"pr", "view"}
	if number > 0 {
		args = append(args, strconv.Itoa(number))
	}
	args = append(args, "--web")
	_, err := runner.Run(ctx, dir, nil, args...)
	return err
}

func (s *Session) List(ctx context.Context, _ review.Diff) ([]review.StoredComment, error) {
	reviewID, err := s.pendingReview(ctx)
	if err != nil || reviewID == 0 {
		return nil, err
	}
	var comments []apiComment
	if err := s.api(ctx, "GET", s.endpoint("reviews/%d/comments", reviewID), nil, &comments); err != nil {
		return nil, err
	}
	result := make([]review.StoredComment, 0, len(comments))
	for _, comment := range comments {
		result = append(result, storedComment(comment))
	}
	return result, nil
}

func (s *Session) Save(ctx context.Context, stored review.StoredComment, _ review.Diff) (review.StoredComment, error) {
	if stored.ID != "" {
		var response struct {
			Update struct {
				Comment graphQLComment `json:"pullRequestReviewComment"`
			} `json:"updatePullRequestReviewComment"`
		}
		if err := s.graphql(ctx, updateCommentMutation, map[string]any{
			"pullRequestReviewCommentId": stored.ID,
			"body":                       stored.Comment.Body,
		}, &response); err != nil {
			return review.StoredComment{}, err
		}
		return storedGraphQLComment(response.Update.Comment, commentSide(stored.Comment.Anchor)), nil
	}

	payload, err := newCommentPayload(stored.Comment)
	if err != nil {
		return review.StoredComment{}, err
	}
	if _, err := s.pendingReview(ctx); err != nil {
		return review.StoredComment{}, err
	}
	if s.reviewNodeID == "" {
		var response struct {
			Add struct {
				Review struct {
					ID       string `json:"id"`
					Database int64  `json:"databaseId"`
					Comments struct {
						Nodes []graphQLComment `json:"nodes"`
					} `json:"comments"`
				} `json:"pullRequestReview"`
			} `json:"addPullRequestReview"`
		}
		if err := s.graphql(ctx, createReviewMutation, map[string]any{
			"pullRequestId": s.pullRequestID,
			"threads":       []any{payload},
		}, &response); err != nil {
			return review.StoredComment{}, err
		}
		if len(response.Add.Review.Comments.Nodes) != 1 {
			return review.StoredComment{}, errors.New("GitHub did not return the created review comment")
		}
		s.reviewNodeID = response.Add.Review.ID
		s.reviewID = response.Add.Review.Database
		return storedGraphQLComment(response.Add.Review.Comments.Nodes[0], commentSide(stored.Comment.Anchor)), nil
	}

	payload["pullRequestReviewId"] = s.reviewNodeID
	var response struct {
		Add struct {
			Thread struct {
				Comments struct {
					Nodes []graphQLComment `json:"nodes"`
				} `json:"comments"`
			} `json:"thread"`
		} `json:"addPullRequestReviewThread"`
	}
	if err := s.graphql(ctx, addThreadMutation, payload, &response); err != nil {
		return review.StoredComment{}, err
	}
	if len(response.Add.Thread.Comments.Nodes) != 1 {
		return review.StoredComment{}, errors.New("GitHub did not return the created review comment")
	}
	return storedGraphQLComment(response.Add.Thread.Comments.Nodes[0], commentSide(stored.Comment.Anchor)), nil
}

func (s *Session) Delete(ctx context.Context, stored review.StoredComment, _ review.Diff) error {
	if stored.ID == "" {
		return errors.New("review comment ID is required")
	}
	return s.graphql(ctx, deleteCommentMutation, map[string]any{"id": stored.ID}, nil)
}

func (s *Session) pendingReview(ctx context.Context) (int64, error) {
	if s.reviewID != 0 {
		return s.reviewID, nil
	}
	var reviews []apiReview
	if err := s.api(ctx, "GET", s.endpoint("reviews"), nil, &reviews); err != nil {
		return 0, err
	}
	for _, item := range reviews {
		if item.State == "PENDING" {
			s.reviewID = item.ID
			s.reviewNodeID = item.NodeID
			return item.ID, nil
		}
	}
	return 0, nil
}

func (s *Session) endpoint(format string, args ...any) string {
	return fmt.Sprintf("repos/%s/pulls/%d/"+format, append([]any{s.repo, s.number}, args...)...)
}

func (s *Session) api(ctx context.Context, method, endpoint string, payload any, result any) error {
	args := []string{"api", "--method", method, endpoint}
	var input []byte
	var err error
	if payload != nil {
		input, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode GitHub request: %w", err)
		}
		args = append(args, "--input", "-")
	}
	out, err := s.runner.Run(ctx, s.dir, input, args...)
	if err != nil {
		return err
	}
	if result == nil || len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	if err := json.Unmarshal(out, result); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (s *Session) graphql(ctx context.Context, query string, input map[string]any, result any) error {
	request, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]any{"input": input},
	})
	if err != nil {
		return fmt.Errorf("encode GitHub GraphQL request: %w", err)
	}
	out, err := s.runner.Run(ctx, s.dir, request, "api", "graphql", "--input", "-")
	if err != nil {
		return err
	}
	var response graphQLResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return fmt.Errorf("decode GitHub GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("GitHub GraphQL: %s", response.Errors[0].Message)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(response.Data, result); err != nil {
		return fmt.Errorf("decode GitHub GraphQL data: %w", err)
	}
	return nil
}

func newCommentPayload(comment review.Comment) (map[string]any, error) {
	anchor := comment.Anchor
	payload := map[string]any{
		"body": comment.Body,
		"path": anchor.File,
	}
	var hasAdded, hasRemoved bool
	for _, line := range anchor.QuotedLines {
		hasAdded = hasAdded || strings.HasPrefix(line, "+")
		hasRemoved = hasRemoved || strings.HasPrefix(line, "-")
	}
	if hasAdded && hasRemoved {
		return nil, errors.New("GitHub review comments cannot span added and removed lines")
	}
	switch {
	case !hasRemoved && anchor.NewEnd > 0:
		payload["line"] = anchor.NewEnd
		payload["side"] = "RIGHT"
		if anchor.NewStart > 0 && anchor.NewStart != anchor.NewEnd {
			payload["startLine"] = anchor.NewStart
			payload["startSide"] = "RIGHT"
		}
	case anchor.OldEnd > 0:
		payload["line"] = anchor.OldEnd
		payload["side"] = "LEFT"
		if anchor.OldStart > 0 && anchor.OldStart != anchor.OldEnd {
			payload["startLine"] = anchor.OldStart
			payload["startSide"] = "LEFT"
		}
	default:
		return nil, errors.New("review comment requires a changed line")
	}
	return payload, nil
}

func storedComment(comment apiComment) review.StoredComment {
	return makeStoredComment(comment.NodeID, comment.Body, comment.Path, comment.DiffHunk, comment.Line, comment.StartLine, comment.Side, comment.StartSide, comment.OriginalLine, comment.OriginalStartLine)
}

func storedGraphQLComment(comment graphQLComment, side string) review.StoredComment {
	return makeStoredComment(comment.ID, comment.Body, comment.Path, comment.DiffHunk, comment.Line, comment.StartLine, side, side, comment.OriginalLine, comment.OriginalStartLine)
}

func commentSide(anchor review.Anchor) string {
	if anchor.OldEnd > 0 {
		return "LEFT"
	}
	return "RIGHT"
}

func makeStoredComment(id, body, path, diffHunk string, line, start int, side, startSide string, originalLine, originalStart int) review.StoredComment {
	if line == 0 {
		line = originalLine
	}
	if start == 0 {
		start = originalStart
	}
	if start == 0 {
		start = line
	}
	anchor := review.Anchor{
		File: path,
		Hunk: diffHunk,
	}
	if side == "" {
		side = startSide
	}
	if side == "LEFT" {
		anchor.OldStart = start
		anchor.OldEnd = line
	} else {
		anchor.NewStart = start
		anchor.NewEnd = line
	}
	return review.StoredComment{
		ID: id,
		Comment: review.Comment{
			Anchor: anchor,
			Body:   body,
		},
	}
}

const commentFields = `
	id
	body
	path
	diffHunk
	line
	startLine
	originalLine
	originalStartLine
`

const createReviewMutation = `mutation($input: AddPullRequestReviewInput!) {
	addPullRequestReview(input: $input) {
		pullRequestReview {
			id
			databaseId
			comments(first: 1) { nodes {` + commentFields + `} }
		}
	}
}`

const addThreadMutation = `mutation($input: AddPullRequestReviewThreadInput!) {
	addPullRequestReviewThread(input: $input) {
		thread { comments(first: 1) { nodes {` + commentFields + `} } }
	}
}`

const updateCommentMutation = `mutation($input: UpdatePullRequestReviewCommentInput!) {
	updatePullRequestReviewComment(input: $input) {
		pullRequestReviewComment {` + commentFields + `}
	}
}`

const deleteCommentMutation = `mutation($input: DeletePullRequestReviewCommentInput!) {
	deletePullRequestReviewComment(input: $input) { clientMutationId }
}`
