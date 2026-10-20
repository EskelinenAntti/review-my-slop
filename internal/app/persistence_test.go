package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreBackedCommentsStayScopedToRepository(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	other, err := store.Add(comments.Comment{
		ID: "other", Repository: "/other", Body: "unrelated",
	})
	testutil.NoError(t, err)
	p := modelPatch()
	m, err := newWithStore(
		context.Background(), store, p, nil, defaultDimensions,
	)
	testutil.NoError(t, err)

	// Switching patch kinds does not change the persistence scope.
	p.Kind = patch.Branch
	m.updatePatch(p)
	saved, err := m.save(comments.Comment{
		Repository: "/ignored", Body: "new comment",
		Anchor: comments.Anchor{FilePath: "main.go"},
	})
	testutil.NoError(t, err)
	testutil.RequireEqual(t, saved.Repository, p.Root, "saved repository")
	if saved.ID == "" || saved.CreatedAt.IsZero() {
		t.Fatal("save did not return the assigned identity and timestamp")
	}

	saved.Body = "updated comment"
	saved.Repository = "/ignored"
	updated, err := m.save(saved)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, updated.Repository, p.Root, "updated repository")
	testutil.RequireEqual(t, updated.ID, saved.ID, "updated identity")
	pending, err := m.load()
	testutil.NoError(t, err)
	if !reflect.DeepEqual(pending, []comments.Comment{updated}) {
		t.Fatalf("loaded comments = %#v, want %#v", pending, updated)
	}

	// Delete uses the session repository, not the input comment's repository.
	testutil.NoError(t, m.delete(saved))
	pending, err = m.load()
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(pending), 0, "comments after deletion")
	remaining, err := store.List(other.Repository)
	testutil.NoError(t, err)
	if !reflect.DeepEqual(remaining, []comments.Comment{other}) {
		t.Fatalf("unrelated comments changed: %#v", remaining)
	}
}

func TestStoreBackedLayoutPreferences(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	testutil.NoError(t, settings.Save(settings.Preferences{SideBySide: true}))
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := newWithStore(
		context.Background(), store, modelPatch(), nil, defaultDimensions,
	)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"stored layout preference")

	testutil.NoError(t, m.saveLayout(false))
	preferences, err := settings.Load()
	testutil.NoError(t, err)
	testutil.RequireEqual(t, preferences.SideBySide, false,
		"updated layout preference")
}

func TestStoreBackedModelRejectsUnreadablePreferences(t *testing.T) {
	config := filepath.Join(t.TempDir(), "not-a-directory")
	testutil.NoError(t, os.WriteFile(config, nil, 0600))
	t.Setenv("XDG_CONFIG_HOME", config)
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	_, err := newWithStore(
		context.Background(), store, modelPatch(), nil, defaultDimensions,
	)
	if err == nil {
		t.Fatal("expected preference loading error")
	}
}
