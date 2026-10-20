package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
