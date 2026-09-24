package ui

import "github.com/eskelinenantti/review-my-slop/internal/patch"

type View = *diffView

func NewUnifiedView(p patch.Patch, dark bool) View {
	return newDiffView(p, dark, false)
}

func NewSideBySideView(p patch.Patch, dark bool) View {
	return newDiffView(p, dark, true)
}
