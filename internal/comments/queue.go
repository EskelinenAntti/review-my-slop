package comments

import (
	"errors"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Draft struct {
	ID     string
	Anchor patch.Anchor
	Body   string
}

type Queue struct {
	store      Store
	repository string
}

func Bind(store Store, repository string) Queue {
	return Queue{store: store, repository: repository}
}

func (q Queue) List() ([]Comment, error) {
	return q.store.List(q.repository)
}

func (q Queue) Save(draft Draft) (Comment, error) {
	if q.repository == "" {
		return Comment{}, errors.New("comment requires a repository")
	}
	comment := Comment{ID: draft.ID, Repository: q.repository, Anchor: draft.Anchor, Body: draft.Body}
	if draft.ID == "" {
		return q.store.Add(comment)
	}
	items, err := q.store.List(q.repository)
	if err != nil {
		return Comment{}, err
	}
	for _, existing := range items {
		if existing.ID != draft.ID {
			continue
		}
		comment.CreatedAt = existing.CreatedAt
		if err := q.store.Update(comment); err != nil {
			return Comment{}, err
		}
		return comment, nil
	}
	return Comment{}, errors.New("comment is no longer in the comments")
}

func (q Queue) Delete(id string) error {
	return q.store.Delete(q.repository, id)
}

func (q Queue) Acknowledge(snapshot []Comment) error {
	ids := make([]string, 0, len(snapshot))
	for _, comment := range snapshot {
		if comment.Repository == q.repository {
			ids = append(ids, comment.ID)
		}
	}
	return q.store.Acknowledge(q.repository, ids)
}
