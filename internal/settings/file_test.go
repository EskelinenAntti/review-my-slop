package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestReplaceSettingsFilePreservesBytes(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{"empty", ""},
		{"raw bytes", "arbitrary bytes\x00without a newline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings", "ui.json")
			testutil.NoError(t, replaceSettingsFile(path, []byte("old data")))
			testutil.NoError(t, os.Chmod(path, 0644))
			testutil.NoError(t, replaceSettingsFile(path, []byte(tc.data)))

			got, err := os.ReadFile(path)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, string(got), tc.data,
				"replacement changed data")
			info, err := os.Stat(path)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, info.Mode().Perm(), os.FileMode(0600),
				"replacement did not secure file permissions")
			entries, err := os.ReadDir(filepath.Dir(path))
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(entries), 1,
				"replacement left temporary files: %v", entries)
		})
	}
}
