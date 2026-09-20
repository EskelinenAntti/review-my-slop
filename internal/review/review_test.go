package review

import (
	"bytes"
	"context"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type fakePatchSource struct {
	current patch.Patch
	root    string
}

func (f fakePatchSource) Root(context.Context, string) (string, error) { return f.root, nil }
func (f fakePatchSource) Load(context.Context, string) (patch.Patch, error) {
	return f.current, nil
}
func (f fakePatchSource) LoadBranch(context.Context, string, string) (patch.Patch, error) {
	return f.current, nil
}
func (f fakePatchSource) DefaultBranch(context.Context, string) (string, error) { return "main", nil }

type fakeCommentStore struct {
	items []comments.Comment
}

func (f *fakeCommentStore) Add(comment comments.Comment) (comments.Comment, error) {
	comment.ID = "new"
	f.items = append(f.items, comment)
	return comment, nil
}
func (f *fakeCommentStore) List(repository string) ([]comments.Comment, error) {
	var result []comments.Comment
	for _, comment := range f.items {
		if comment.Repository == repository {
			result = append(result, comment)
		}
	}
	return result, nil
}
func (f *fakeCommentStore) Update(comment comments.Comment) error {
	for index := range f.items {
		if f.items[index].ID == comment.ID {
			f.items[index] = comment
			return nil
		}
	}
	return nil
}
func (f *fakeCommentStore) Delete(repository, id string) error {
	for index, comment := range f.items {
		if comment.Repository == repository && comment.ID == id {
			f.items = append(f.items[:index], f.items[index+1:]...)
			break
		}
	}
	return nil
}
func (f *fakeCommentStore) Acknowledge(repository string, ids []string) error {
	for _, id := range ids {
		_ = f.Delete(repository, id)
	}
	return nil
}

func TestSaveCommentAddsRepositoryContext(t *testing.T) {
	store := &fakeCommentStore{}
	currentReview := New(context.Background(), "/work", fakePatchSource{}, store)
	p := patch.Patch{Repository: "/repo"}

	saved, err := currentReview.SaveComment(comments.Comment{Body: "check this"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Repository != "/repo" || len(store.items) != 1 {
		t.Fatalf("saved = %#v, items = %#v", saved, store.items)
	}
}

func TestExportDoesNotAcknowledge(t *testing.T) {
	store := &fakeCommentStore{items: []comments.Comment{{ID: "one", Repository: "/repo", Body: "check"}}}
	currentReview := New(context.Background(), "/work", fakePatchSource{}, store)
	var output bytes.Buffer

	pending, err := currentReview.ExportComments(&output, patch.Patch{Repository: "/repo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || len(store.items) != 1 {
		t.Fatalf("pending = %#v, stored = %#v", pending, store.items)
	}
	if err := currentReview.Acknowledge(patch.Patch{Repository: "/repo"}, pending); err != nil {
		t.Fatal(err)
	}
	if len(store.items) != 0 {
		t.Fatalf("stored after acknowledgement = %#v", store.items)
	}
}
