package comments

import (
	"errors"
	"io"
)

type Inbox struct {
	store      Store
	repository string
}

func OpenInbox(repository string) (Inbox, error) {
	store, err := OpenDefault()
	if err != nil {
		return Inbox{}, err
	}
	return NewInbox(store, repository)
}

func NewInbox(store Store, repository string) (Inbox, error) {
	if store.Path() == "" {
		return Inbox{}, errors.New("store path is required")
	}
	if repository == "" {
		return Inbox{}, errors.New("repository is required")
	}
	return Inbox{store: store, repository: repository}, nil
}

func (i Inbox) List() ([]Comment, error) {
	return i.store.List(i.repository)
}

func (i Inbox) Save(comment Comment) (Comment, error) {
	comment.Repository = i.repository
	if comment.ID != "" {
		err := i.store.Update(comment)
		return comment, err
	}
	return i.store.Add(comment)
}

func (i Inbox) Delete(comment Comment) error {
	if comment.ID == "" {
		return errors.New("comment ID is required")
	}
	return i.store.Delete(i.repository, comment.ID)
}

func (i Inbox) SideBySide() (bool, error) {
	return i.store.SideBySide()
}

func (i Inbox) SetSideBySide(enabled bool) error {
	return i.store.SetSideBySide(enabled)
}

func (i Inbox) Deliver(output io.Writer) error {
	return i.store.Deliver(i.repository, output)
}
