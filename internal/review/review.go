package review

import (
	"context"
	"io"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// PatchSource is the review-level seam for obtaining the patch under review.
// The Git implementation lives in patch; review only needs these operations.
type PatchSource interface {
	Root(context.Context, string) (string, error)
	Load(context.Context, string) (patch.Patch, error)
	LoadBranch(context.Context, string, string) (patch.Patch, error)
	DefaultBranch(context.Context, string) (string, error)
}

// CommentStore is the review-level seam for comment persistence.
type CommentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
	Acknowledge(string, []string) error
}

// Review coordinates the patch and comments that make up one review.
type Review struct {
	context   context.Context
	directory string
	patches   PatchSource
	comments  CommentStore
}

func New(ctx context.Context, directory string, patches PatchSource, comments CommentStore) Review {
	return Review{context: ctx, directory: directory, patches: patches, comments: comments}
}

func (r Review) Repository() (string, error) {
	return r.patches.Root(r.context, r.directory)
}

func (r Review) Load(branch string) (patch.Patch, error) {
	if branch == "" {
		return r.patches.Load(r.context, r.directory)
	}
	return r.patches.LoadBranch(r.context, r.directory, branch)
}

func (r Review) DefaultBranch() (string, error) {
	return r.patches.DefaultBranch(r.context, r.directory)
}

func (r Review) Comments(p patch.Patch) ([]comments.Comment, error) {
	return r.comments.List(p.Repository)
}

func (r Review) SaveComment(comment comments.Comment, p patch.Patch) (comments.Comment, error) {
	comment.Repository = p.Repository
	if comment.ID != "" {
		if err := r.comments.Update(comment); err != nil {
			return comments.Comment{}, err
		}
		return comment, nil
	}
	return r.comments.Add(comment)
}

func (r Review) DeleteComment(comment comments.Comment, p patch.Patch) error {
	return r.comments.Delete(p.Repository, comment.ID)
}

func (r Review) ExportComments(w io.Writer, p patch.Patch) ([]comments.Comment, error) {
	pending, err := r.Comments(p)
	if err != nil {
		return nil, err
	}
	if err := comments.WritePrompt(w, pending); err != nil {
		return pending, err
	}
	return pending, nil
}

func (r Review) ExportRepository(w io.Writer, repository string) ([]comments.Comment, error) {
	return r.ExportComments(w, patch.Patch{Repository: repository})
}

func (r Review) Acknowledge(p patch.Patch, pending []comments.Comment) error {
	return r.AcknowledgeRepository(p.Repository, pending)
}

func (r Review) AcknowledgeRepository(repository string, pending []comments.Comment) error {
	return r.comments.Acknowledge(repository, commentIDs(pending))
}

func commentIDs(pending []comments.Comment) []string {
	ids := make([]string, len(pending))
	for index, comment := range pending {
		ids[index] = comment.ID
	}
	return ids
}
