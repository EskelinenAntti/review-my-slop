package slop

import (
	"os/exec"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/github"
)

func selectDiffTarget() (diffTarget, <-chan reviewContext) {
	local := diffTarget{Source: sourceLocal, Args: localDiffArgs()}
	refs, err := localChangedLineRefs(local.Args)
	if err != nil || len(refs) > 0 {
		return local, detectReviewContextAsync()
	}

	if pr := github.DetectPR(); pr != nil {
		return diffTarget{
			Source: sourceBranch,
			Args:   []string{pr.Base + "..." + pr.Head},
		}, detectReviewContextForPRAsync(pr)
	}

	if base, ok := branchBaseRef(); ok {
		return diffTarget{
			Source: sourceBranch,
			Args:   []string{base + "...HEAD"},
		}, completedReviewContext(reviewContext{})
	}

	return local, completedReviewContext(reviewContext{})
}

func localDiffArgs() []string {
	if gitRefExists("HEAD") {
		return []string{"HEAD"}
	}
	return nil
}

func detectReviewContextForPRAsync(pr *prContext) <-chan reviewContext {
	ch := make(chan reviewContext, 1)
	go func() {
		ch <- reviewContext{
			PR:    pr,
			Draft: github.DetectPendingReview(pr),
		}
	}()
	return ch
}

func completedReviewContext(context reviewContext) <-chan reviewContext {
	ch := make(chan reviewContext, 1)
	ch <- context
	return ch
}

func branchBaseRef() (string, bool) {
	if ref, ok := gitOutput("symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); ok {
		return ref, true
	}
	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		if gitRefExists(ref) {
			return ref, true
		}
	}
	return "", false
}

func gitRefExists(ref string) bool {
	_, ok := gitOutput("rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return ok
}

func gitOutput(args ...string) (string, bool) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(string(out))
	return text, text != ""
}
