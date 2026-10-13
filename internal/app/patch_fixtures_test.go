package app

import (
	"fmt"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func singleHunkPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{{
			DisplayPath: "main.go",
			OldPath:     "main.go",
			NewPath:     "main.go",
			Hunks: []patch.Hunk{{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []patch.Line{
					{
						Kind: patch.Context, Text: "keep()",
						OldNumber: 1, NewNumber: 1,
					},
					{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
					{Kind: patch.Addition, Text: "new()", NewNumber: 2},
				},
			}},
		}},
	}
}

func multiHunkPatch() patch.Patch {
	return patch.Patch{
		Root: "/repo",
		Files: []patch.File{{
			DisplayPath: "main.go",
			OldPath:     "main.go",
			NewPath:     "main.go",
			OldSource:   "package main\nold()\nkeep()\n",
			NewSource:   "package main\nnew()\nkeep()\nmore()\n",
			Hunks: []patch.Hunk{
				{
					Header: "@@ -1,3 +1,3 @@",
					Lines: []patch.Line{
						{
							Kind: patch.Context, Text: "package main",
							OldNumber: 1, NewNumber: 1,
						},
						{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
						{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						{
							Kind: patch.Context, Text: "keep()",
							OldNumber: 3, NewNumber: 3,
						},
					},
				},
				{
					Header: "@@ -3,1 +3,2 @@",
					Lines: []patch.Line{
						{
							Kind: patch.Context, Text: "keep()",
							OldNumber: 3, NewNumber: 3,
						},
						{Kind: patch.Addition, Text: "more()", NewNumber: 4},
					},
				},
			},
		}},
	}
}

func longContextPatch() patch.Patch {
	lines := make([]patch.Line, 30)
	for index := range lines {
		lines[index] = patch.Line{
			Kind: patch.Context,
			Text: fmt.Sprintf(
				"line %d %s", index, strings.Repeat("x", 80),
			),
			OldNumber: patch.LineNumber(index + 1),
			NewNumber: patch.LineNumber(index + 1),
		}
	}
	return patch.Patch{Files: []patch.File{{
		DisplayPath: "long.go",
		Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
	}}}
}
