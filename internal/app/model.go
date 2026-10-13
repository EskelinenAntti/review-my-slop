package app

import (
	"context"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

type saveCommentToStoreFunc func(
	comments.Comment,
	patch.Patch,
) (comments.Comment, error)

type deleteCommentFromStoreFunc func(comments.Comment, patch.Patch) error

type reloadCommentsFunc func() ([]comments.Comment, error)

type refreshPatchFunc func(patch.Kind) (patch.Patch, error)

type saveSideBySideFunc func(bool) error

// commentStore supplies the persistence operations used by the terminal client.
type commentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
}

type size struct {
	Width  int
	Height int
}

type initialLayout struct {
	SideBySide     bool
	SaveSideBySide saveSideBySideFunc
	size           size
}

type mode uint8

const (
	modeBrowse mode = iota
	modeComments
	modeHelp
	modeSearch
)

const horizontalScrollStep = 4

const verticalScrollStep = 3

var defaultSize = size{Width: 80, Height: 30}

// commentState pairs stored comments with the revision used to reject stale
// loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentEdit struct {
	body   string
	index  int
	anchor comments.Anchor
}

type model struct {
	ctx context.Context

	patchView   *diffscreen.View
	commentView *commentscreen.View
	width       int
	height      int
	mode        mode
	keys        keymap.Matcher

	currentPatch     patch.Patch
	kind             patch.Kind
	patchViewOptions diffscreen.Options
	comments         commentState
	edit             commentEdit

	saveCommentToStore     saveCommentToStoreFunc
	deleteCommentFromStore deleteCommentFromStoreFunc
	reloadComments         reloadCommentsFunc
	refreshPatch           refreshPatchFunc
	saveLayout             saveSideBySideFunc
	err                    error
	quitting               bool
}
