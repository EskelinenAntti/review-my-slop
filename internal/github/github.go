package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type PR struct {
	Number int    `json:"number"`
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Head   string `json:"head"`
	Base   string `json:"base"`
}

type Draft struct {
	Active bool
	ID     string
	Count  int
}

type ReviewThread struct {
	ID       string
	File     string
	Line     int
	Side     string
	Pending  bool
	Comments []ReviewComment
}

type ReviewComment struct {
	ID        string
	Body      string
	Author    string
	CreatedAt string
	Pending   bool
}

type LineRange struct {
	Start Line
	End   Line
}

type Line struct {
	File string
	Line int
	Side string
}

func DetectPR() *PR {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	cmd := exec.Command("gh", "pr", "view",
		"--json", "id,number,headRefOid,headRepository,headRepositoryOwner,baseRefOid",
		"--jq", `{"id": .id, "number": .number, "owner": .headRepositoryOwner.login, "repo": .headRepository.name, "head": .headRefOid, "base": .baseRefOid}`,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pr PR
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil
	}
	if pr.Number == 0 || pr.ID == "" || pr.Owner == "" || pr.Repo == "" || pr.Head == "" || pr.Base == "" {
		return nil
	}
	return &pr
}

func PostReviewComment(pr *PR, lineRange LineRange, body string) error {
	data, err := json.Marshal(ReviewCommentPayload(pr, lineRange, body))
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", pr.Owner, pr.Repo, pr.Number)
	cmd := exec.Command("gh", "api", "-X", "POST", endpoint, "--input", "-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh api failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func ReviewCommentPayload(pr *PR, lineRange LineRange, body string) map[string]any {
	payload := map[string]any{
		"body":      body,
		"path":      lineRange.End.File,
		"line":      lineRange.End.Line,
		"side":      Side(lineRange.End.Side),
		"commit_id": pr.Head,
	}
	if lineRange.Start.Line != lineRange.End.Line {
		payload["start_line"] = lineRange.Start.Line
		payload["start_side"] = Side(lineRange.Start.Side)
	}
	return payload
}

func CreatePendingReview(pr *PR) (string, error) {
	const query = `
mutation($pullRequestID: ID!, $commitOID: GitObjectID!) {
  addPullRequestReview(input: {pullRequestId: $pullRequestID, commitOID: $commitOID}) {
    pullRequestReview {
      id
    }
  }
}`
	var response struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				ID string `json:"id"`
			} `json:"pullRequestReview"`
		} `json:"addPullRequestReview"`
	}
	err := graphQL(query, map[string]any{
		"pullRequestID": pr.ID,
		"commitOID":     pr.Head,
	}, &response)
	if err != nil {
		return "", err
	}
	if response.AddPullRequestReview.PullRequestReview.ID == "" {
		return "", errors.New("GitHub did not return a pending review id")
	}
	return response.AddPullRequestReview.PullRequestReview.ID, nil
}

func DetectPendingReview(pr *PR) Draft {
	const query = `
query($pullRequestID: ID!) {
  viewer {
    login
  }
  node(id: $pullRequestID) {
    ... on PullRequest {
      reviews(states: PENDING, first: 20) {
        nodes {
          id
          author {
            login
          }
          comments(first: 1) {
            totalCount
          }
        }
      }
    }
  }
}`
	var response struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
		Node struct {
			Reviews struct {
				Nodes []struct {
					ID     string `json:"id"`
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					Comments struct {
						TotalCount int `json:"totalCount"`
					} `json:"comments"`
				} `json:"nodes"`
			} `json:"reviews"`
		} `json:"node"`
	}
	if err := graphQL(query, map[string]any{"pullRequestID": pr.ID}, &response); err != nil {
		return Draft{}
	}
	for _, review := range response.Node.Reviews.Nodes {
		if review.ID == "" || review.Author.Login != response.Viewer.Login {
			continue
		}
		return Draft{
			Active: true,
			ID:     review.ID,
			Count:  review.Comments.TotalCount,
		}
	}
	return Draft{}
}

func DetectReviewThreads(pr *PR) []ReviewThread {
	const query = `
query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100) {
        nodes {
          id
          path
          line
          diffSide
          isResolved
          comments(first: 20) {
            nodes {
              id
              body
              createdAt
              state
              author {
                login
              }
              pullRequestReview {
                state
              }
            }
          }
        }
      }
    }
  }
}`
	var response struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []struct {
						ID         string `json:"id"`
						Path       string `json:"path"`
						Line       int    `json:"line"`
						DiffSide   string `json:"diffSide"`
						IsResolved bool   `json:"isResolved"`
						Comments   struct {
							Nodes []struct {
								ID        string `json:"id"`
								Body      string `json:"body"`
								CreatedAt string `json:"createdAt"`
								State     string `json:"state"`
								Author    struct {
									Login string `json:"login"`
								} `json:"author"`
								PullRequestReview struct {
									State string `json:"state"`
								} `json:"pullRequestReview"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	err := graphQL(query, map[string]any{
		"owner":  pr.Owner,
		"repo":   pr.Repo,
		"number": pr.Number,
	}, &response)
	if err != nil {
		return nil
	}
	var threads []ReviewThread
	for _, node := range response.Repository.PullRequest.ReviewThreads.Nodes {
		if node.IsResolved || node.ID == "" || node.Path == "" || node.Line == 0 {
			continue
		}
		thread := ReviewThread{
			ID:   node.ID,
			File: node.Path,
			Line: node.Line,
			Side: lineSide(node.DiffSide),
		}
		for _, commentNode := range node.Comments.Nodes {
			pending := commentNode.State == "PENDING" || commentNode.PullRequestReview.State == "PENDING"
			thread.Pending = thread.Pending || pending
			thread.Comments = append(thread.Comments, ReviewComment{
				ID:        commentNode.ID,
				Body:      commentNode.Body,
				Author:    commentNode.Author.Login,
				CreatedAt: commentNode.CreatedAt,
				Pending:   pending,
			})
		}
		threads = append(threads, thread)
	}
	return threads
}

func AddPendingReviewComment(reviewID string, lineRange LineRange, body string) error {
	const query = `
mutation($reviewID: ID!, $body: String!, $path: String!, $line: Int!, $side: DiffSide!, $startLine: Int, $startSide: DiffSide) {
  addPullRequestReviewThread(input: {pullRequestReviewId: $reviewID, body: $body, path: $path, line: $line, side: $side, startLine: $startLine, startSide: $startSide}) {
    thread {
      id
    }
  }
}`
	var response struct {
		AddPullRequestReviewThread struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		} `json:"addPullRequestReviewThread"`
	}
	return graphQL(query, ReviewThreadVariables(reviewID, lineRange, body), &response)
}

func UpdatePendingReviewComment(commentID string, body string) error {
	const query = `
mutation($commentID: ID!, $body: String!) {
  updatePullRequestReviewComment(input: {pullRequestReviewCommentId: $commentID, body: $body}) {
    pullRequestReviewComment {
      id
    }
  }
}`
	var response struct {
		UpdatePullRequestReviewComment struct {
			PullRequestReviewComment struct {
				ID string `json:"id"`
			} `json:"pullRequestReviewComment"`
		} `json:"updatePullRequestReviewComment"`
	}
	return graphQL(query, map[string]any{
		"commentID": commentID,
		"body":      strings.TrimSpace(body),
	}, &response)
}

func SubmitPendingReview(reviewID string, body string) error {
	const query = `
mutation($reviewID: ID!, $body: String, $event: PullRequestReviewEvent!) {
  submitPullRequestReview(input: {pullRequestReviewId: $reviewID, body: $body, event: $event}) {
    pullRequestReview {
      id
    }
  }
}`
	var response struct {
		SubmitPullRequestReview struct {
			PullRequestReview struct {
				ID string `json:"id"`
			} `json:"pullRequestReview"`
		} `json:"submitPullRequestReview"`
	}
	return graphQL(query, map[string]any{
		"reviewID": reviewID,
		"body":     strings.TrimSpace(body),
		"event":    "COMMENT",
	}, &response)
}

func DeletePendingReview(reviewID string) error {
	const query = `
mutation($reviewID: ID!) {
  deletePullRequestReview(input: {pullRequestReviewId: $reviewID}) {
    pullRequestReview {
      id
    }
  }
}`
	var response struct {
		DeletePullRequestReview struct {
			PullRequestReview struct {
				ID string `json:"id"`
			} `json:"pullRequestReview"`
		} `json:"deletePullRequestReview"`
	}
	return graphQL(query, map[string]any{"reviewID": reviewID}, &response)
}

func ReviewThreadVariables(reviewID string, lineRange LineRange, body string) map[string]any {
	payload := map[string]any{
		"reviewID": reviewID,
		"body":     body,
		"path":     lineRange.End.File,
		"line":     lineRange.End.Line,
		"side":     Side(lineRange.End.Side),
	}
	if lineRange.Start.Line != lineRange.End.Line {
		payload["startLine"] = lineRange.Start.Line
		payload["startSide"] = Side(lineRange.Start.Side)
	}
	return payload
}

func graphQL(query string, variables map[string]any, target any) error {
	request := map[string]any{
		"query":     query,
		"variables": variables,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "api", "graphql", "--input", "-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh api graphql failed: %s", strings.TrimSpace(string(out)))
	}

	var response struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return err
	}
	if len(response.Errors) > 0 {
		var messages []string
		for _, graphQLError := range response.Errors {
			messages = append(messages, graphQLError.Message)
		}
		return fmt.Errorf("gh api graphql failed: %s", strings.Join(messages, ", "))
	}
	if len(response.Data) == 0 {
		return errors.New("gh api graphql returned no data")
	}
	return json.Unmarshal(response.Data, target)
}

func Side(side string) string {
	if side == "old" {
		return "LEFT"
	}
	return "RIGHT"
}

func lineSide(side string) string {
	if side == "LEFT" {
		return "old"
	}
	return "new"
}
