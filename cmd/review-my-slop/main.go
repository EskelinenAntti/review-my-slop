package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/gitdiff"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/pullrequest"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCode(ctx)
	}
	switch args[0] {
	case "code":
		if len(args) != 1 {
			return usageError()
		}
		return runCode(ctx)
	case "comments":
		if len(args) != 1 {
			return usageError()
		}
		return runComments(ctx, output)
	case "pr":
		if len(args) != 2 {
			return usageError()
		}
		number, err := strconv.Atoi(args[1])
		if err != nil || number <= 0 {
			return fmt.Errorf("invalid pull request number %q", args[1])
		}
		return runPR(ctx, number)
	default:
		return fmt.Errorf("unknown subcommand %q; %w", args[0], usageError())
	}
}

func usageError() error {
	return fmt.Errorf("usage: review-my-slop [code|comments|pr NUMBER]")
}

func runCode(ctx context.Context) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	loaded, err := (gitdiff.Loader{}).Load(ctx, current)
	if err != nil {
		return err
	}

	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	sideBySide, err := store.SideBySide()
	if err != nil {
		return err
	}
	backend := localStore{store: store}
	return runReview(ctx, current, gitdiff.Loader{}, loaded, backend, true, sideBySide, store.SetSideBySide, func() error {
		return pullrequest.OpenBrowser(ctx, current, 0, nil)
	})
}

func runPR(ctx context.Context, number int) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	settings, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	sideBySide, err := settings.SideBySide()
	if err != nil {
		return err
	}
	loader := gitdiff.Loader{}
	store, err := pullrequest.Open(ctx, current, number, nil)
	if err != nil {
		return err
	}
	loaded, err := loader.LoadBranch(ctx, current, store.Base)
	if err != nil {
		return err
	}
	return runReview(ctx, current, loader, loaded, store, false, sideBySide, settings.SetSideBySide, func() error {
		return pullrequest.OpenBrowser(ctx, current, number, store.Runner)
	})
}

type reviewStore interface {
	List(context.Context, review.Diff) ([]review.StoredComment, error)
	Save(context.Context, review.StoredComment, review.Diff) (review.StoredComment, error)
	Delete(context.Context, review.StoredComment, review.Diff) error
}

func runReview(ctx context.Context, current string, loader gitdiff.Loader, loaded review.Diff, store reviewStore, allowParents bool, sideBySide bool, saveSideBySide tui.SaveSideBySideFunc, openPR tui.OpenPullRequestFunc) error {
	comments, err := store.List(ctx, loaded)
	if err != nil {
		return err
	}
	model := tui.New(loaded, comments, func(stored review.StoredComment, diff review.Diff) (review.StoredComment, error) {
		return store.Save(ctx, stored, diff)
	})
	model.SetSideBySide(sideBySide, saveSideBySide)
	model.SetDelete(func(stored review.StoredComment, diff review.Diff) error {
		return store.Delete(ctx, stored, diff)
	})
	model.SetOpenPullRequest(openPR)
	if allowParents {
		parents, err := loader.ParentBranches(ctx, current)
		if err != nil {
			return err
		}
		model.SetParents(parents)
		model.SetRefresh(func(parent string) (review.Diff, error) {
			if parent != "" {
				return loader.LoadBranch(ctx, current, parent)
			}
			return loader.Load(ctx, current)
		})
	} else {
		model.SetRefresh(func(string) (review.Diff, error) {
			return loader.LoadBranch(ctx, current, loaded.Base)
		})
	}
	program := tea.NewProgram(model)
	_, err = program.Run()
	return err
}

type localStore struct {
	store inbox.Store
}

func (s localStore) List(_ context.Context, diff review.Diff) ([]review.StoredComment, error) {
	return s.store.ListComments(diff.Repository)
}

func (s localStore) Save(_ context.Context, stored review.StoredComment, diff review.Diff) (review.StoredComment, error) {
	if stored.ID != "" {
		return stored, s.store.UpdateComment(diff.Repository, stored)
	}
	message := inbox.Message{
		ID:              fmt.Sprintf("%d", time.Now().UnixNano()),
		Repository:      diff.Repository,
		DiffFingerprint: diff.Fingerprint,
		CreatedAt:       time.Now().UTC(),
		Comment:         stored.Comment,
	}
	if err := s.store.Put(message); err != nil {
		return review.StoredComment{}, err
	}
	stored.ID = message.ID
	return stored, nil
}

func (s localStore) Delete(_ context.Context, stored review.StoredComment, diff review.Diff) error {
	return s.store.DeleteComment(diff.Repository, stored)
}

func runComments(ctx context.Context, output io.Writer) error {
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCommentsAt(ctx, current, output)
}

func runCommentsAt(ctx context.Context, current string, output io.Writer) error {
	root, err := (gitdiff.Loader{}).Root(ctx, current)
	if err != nil {
		return err
	}
	store, err := inbox.OpenDefault()
	if err != nil {
		return err
	}
	taken, err := store.Peek(root)
	if err != nil {
		return err
	}
	if err := inbox.WritePrompt(output, taken.Messages); err != nil {
		return err
	}
	return store.Delete(taken)
}
