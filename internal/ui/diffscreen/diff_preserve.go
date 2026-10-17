package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor diffCursor
	valid  bool
}

// replaceView translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search
// origin.
