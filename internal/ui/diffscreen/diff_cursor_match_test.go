package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCursorIdentitySameHunk(t *testing.T) {
	target := cursorIdentity{
		oldPath: "old.go", newPath: "new.go",
		hunkHeader: "@@ -1,2 +1,3 @@", valid: true,
	}
	for _, tc := range []struct {
		name  string
		other cursorIdentity
		want  bool
	}{
		{"identical", target, true},
		{"invalid", cursorIdentity{}, false},
		{"invalid with matching source", cursorIdentity{
			oldPath: target.oldPath, newPath: target.newPath,
			hunkHeader: target.hunkHeader,
		}, false},
		{"old path matches", cursorIdentity{
			oldPath: target.oldPath, newPath: "renamed.go",
			hunkHeader: target.hunkHeader, valid: true,
		}, true},
		{"new path matches", cursorIdentity{
			oldPath: "previous.go", newPath: target.newPath,
			hunkHeader: target.hunkHeader, valid: true,
		}, true},
		{"different file", cursorIdentity{
			oldPath: "other.go", newPath: "other.go",
			hunkHeader: target.hunkHeader, valid: true,
		}, false},
		{"different hunk", cursorIdentity{
			oldPath: target.oldPath, newPath: target.newPath,
			hunkHeader: "@@ -10,2 +10,3 @@", valid: true,
		}, false},
		{"empty paths", cursorIdentity{
			hunkHeader: target.hunkHeader, valid: true,
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.RequireEqual(t, target.sameHunk(tc.other), tc.want,
				"matching hunk")
			testutil.RequireEqual(t, tc.other.sameHunk(target), tc.want,
				"matching hunk is symmetric")
		})
	}
}
