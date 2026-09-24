package review

import (
	"context"
	"io"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// Review coordinates the patch and comments that make up one review.
type Review struct {
	context   context.Context
	directory string
	patches   patch.Loader
	comments  comments.Store
}

func New(ctx context.Context, directory string, patches patch.Loader, comments comments.Store) Review {
	return Review{context: ctx, directory: directory, patches: patches, comments: comments}
}

func (r Review) Load(branch string) (patch.Patch, error) {
	if branch == "" {
		return r.patches.Load(r.context, r.directory)
	}
	return r.patches.LoadBranch(r.context, r.directory, branch)
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

func (r Review) AcknowledgeRepository(repository string, pending []comments.Comment) error {
	ids := make([]string, len(pending))
	for index, comment := range pending {
		ids[index] = comment.ID
	}
	return r.comments.Acknowledge(repository, ids)
}
