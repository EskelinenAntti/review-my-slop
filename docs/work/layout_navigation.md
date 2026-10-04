# Layout and navigation work

- Owner: layout/navigation worker
- Paths: `internal/layout`, `internal/navigation`, this status file
- Status: implementation complete; targeted behavior tests pass
- Validation: `/workspace/toolchains/go/bin/go test ./internal/layout ./internal/navigation`
- Migrated assertions from `diff_view_test.go`: `TestUnifiedNavigationSearchAndFileJumps`,
  `TestSplitPairsChangeBlocksAndSupportsEmptyPanes`, `TestSelectionLinesAndAnchor`
  (range assertions; Anchor construction belongs to core),
  `TestViewportAlignmentResizeAndScrolling`, `TestViewportProgressUsesVisibleBottom`,
  `TestHalfPageScrollingMovesCursorToFileBoundaries`,
  `TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates`,
  `TestSplitViewWithOnlyDeletionsStartsInLeftPane`, and
  `TestFindCursorFallsBackNearRemovedLine`.
- Migrated assertions from `diff_behavior_test.go`:
  `TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate`,
  `TestSplitSelectionOnlyIncludesActivePane`,
  `TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets`,
  `TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty`,
  `TestSplitVerticalMovementSkipsEmptyActivePane`,
  `TestSplitVerticalMovementAndHalfPageUseVisualRows`,
  `TestKeepVisibleAccountsForStickyFileHeader`, and
  `TestHorizontalScrollStartAndEndClamp`. Renderer-owned divider, ANSI, exact
  terminal width, syntax coloring, and style assertions remain with the render owner.
- Migrated assertions from `diff_preserve_test.go`:
  `TestPreserveTranslatesCursorSelectionAndViewport` and
  `TestPreserveReturnsEmptyStateForEmptyView`.
- Claim commit: `c9f624296754649a2507f50701073537167546e7`
- Implementation commit: `29174bf548efe73a7e5bea216188a6551477a422`
