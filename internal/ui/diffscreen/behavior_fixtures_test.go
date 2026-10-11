package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func unequalChangePatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: fixtureOne,
						Lines: []patch.Line{
							{
								Kind:      patch.Deletion,
								Text:      "d1",
								OldNumber: 1,
							},
							{
								Kind:      patch.Deletion,
								Text:      "d2",
								OldNumber: 2,
							},
							{
								Kind:      patch.Addition,
								Text:      "a1",
								NewNumber: 1,
							},
							{
								Kind:      patch.Addition,
								Text:      "a2",
								NewNumber: 2,
							},
							{
								Kind:      patch.Addition,
								Text:      "a3",
								NewNumber: 3,
							},
							{
								Kind:      patch.Context,
								Text:      "c",
								OldNumber: 3,
								NewNumber: 4,
							},
						},
					},

					{
						Header: fixtureTwo,
						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      "separate",
								NewNumber: 5,
							},
						},
					},
				},
			},
		},
	}
}

func stickyHeaderPatch() patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "first.go",
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "first one",
								OldNumber: 1,
								NewNumber: 1,
							},

							{
								Kind:      patch.Context,
								Text:      "first two",
								OldNumber: 2,
								NewNumber: 2,
							},

							{
								Kind:      patch.Context,
								Text:      "first three",
								OldNumber: 3,
								NewNumber: 3,
							},
						},
					},
				},
			},

			{
				DisplayPath: fixtureSecondGo,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "second one",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
}
