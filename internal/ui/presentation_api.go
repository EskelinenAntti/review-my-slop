package ui

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

// Presentation is the terminal-facing projection of a change set. It owns
// visual rows while exposing semantic navigation operations to the model.
type Presentation = presentation

// NewUnifiedView creates a presentation using one vertically ordered diff.
func NewUnifiedView(changes diff.ChangeSet, dark bool) *Presentation {
	return newPresentation(changes, false, dark)
}

// NewSideBySideView creates a presentation with paired old and new panes.
func NewSideBySideView(changes diff.ChangeSet, dark bool) *Presentation {
	return newPresentation(changes, true, dark)
}

func newPresentation(changes diff.ChangeSet, split, dark bool) *presentation {
	presentation := &presentation{}
	presentation.newView(changes, split, dark)
	return presentation
}

// File returns the semantic file containing cursor.
func (p *presentation) File(cursor Cursor) (diff.File, bool) { return p.file(cursor) }

// Hunk returns the semantic hunk containing cursor.
func (p *presentation) Hunk(cursor Cursor) (diff.Hunk, bool) { return p.hunk(cursor) }

// Line returns the semantic diff line under cursor.
func (p *presentation) Line(cursor Cursor) (diff.Line, bool) { return p.line(cursor) }

// Lines returns the semantic lines in selection order.
func (p *presentation) Lines(selection Selection) []diff.Line {
	lines, _, ok := p.selectedLines(selection)
	if !ok {
		return nil
	}
	return lines
}

// Anchor converts a presentation selection to a repository-independent
// comment anchor.
func (p *presentation) Anchor(selection Selection) (comments.Anchor, error) {
	return p.anchor(selection)
}

// ViewportProgress reports the percentage of presentation rows below the
// visible area.
func (p *presentation) ViewportProgress(viewport Viewport) int { return p.Progress(viewport) }
