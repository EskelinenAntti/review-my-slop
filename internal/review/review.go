package review

import (
	"context"
	"io"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// CommentStore is the review-level seam for comment persistence.
type CommentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
	Acknowledge(string, []string) error
}

// Actions is the narrow interface a human-facing UI needs to drive a review.
type Actions interface {
	Load(patch.Kind) (patch.Patch, error)
	Comments(patch.Patch) ([]comments.Comment, error)
	SaveComment(comments.Comment, patch.Patch) (comments.Comment, error)
	DeleteComment(comments.Comment, patch.Patch) error
}

// Review coordinates the patch and comments that make up one review.
type Review struct {
	context  context.Context
	get      func(context.Context, patch.Kind) (patch.Patch, error)
	comments CommentStore
}

var _ Actions = Review{}

func New(ctx context.Context, get func(context.Context, patch.Kind) (patch.Patch, error), comments CommentStore) Review {
	return Review{context: ctx, get: get, comments: comments}
}

func (r Review) Load(kind patch.Kind) (patch.Patch, error) {
	return r.get(r.context, kind)
}

func (r Review) Comments(p patch.Patch) ([]comments.Comment, error) {
	return r.comments.List(p.Root)
}

func (r Review) SaveComment(comment comments.Comment, p patch.Patch) (comments.Comment, error) {
	comment.Repository = p.Root
	if comment.ID != "" {
		if err := r.comments.Update(comment); err != nil {
			return comments.Comment{}, err
		}
		return comment, nil
	}
	return r.comments.Add(comment)
}

func (r Review) DeleteComment(comment comments.Comment, p patch.Patch) error {
	return r.comments.Delete(p.Root, comment.ID)
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

func (r Review) Acknowledge(p patch.Patch, pending []comments.Comment) error {
	ids := make([]string, len(pending))
	for index, comment := range pending {
		ids[index] = comment.ID
	}
	return r.comments.Acknowledge(p.Root, ids)
}
