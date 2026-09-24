package review

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Review struct {
	Patches patch.Loader
	Store   comments.Store
}

func (r Review) SaveComment(comment comments.Comment, p patch.Patch) (comments.Comment, error) {
	comment.Repository = p.Repository
	if comment.ID == "" {
		return r.Store.Add(comment)
	}
	if err := r.Store.Update(comment); err != nil {
		return comments.Comment{}, err
	}
	return comment, nil
}

func (r Review) DeleteComment(comment comments.Comment, p patch.Patch) error {
	return r.Store.Delete(p.Repository, comment.ID)
}
