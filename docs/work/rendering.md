# Rendering work claim

- Owner: rendering worker
- Paths: `internal/render`, `docs/work/rendering.md`
- Status: implementation complete
- Validation: `/workspace/toolchains/go/bin/go test ./internal/render` passed

## Migrated behavior coverage

`internal/render/terminal_test.go` retains the meaningful assertions from the
old UI tests. `TestLicenseHighlightFixture` and
`TestHighlightAdaptsToTerminalBackgroundAndKeepsSourceText` cover highlighting;
`TestFileHeaderSticksWithoutCoveringDiffRows` moved to
`TestTerminalStickyHeaderDoesNotCoverRows`; split and unified scrolling tests
moved to `TestTerminalKeepsSplitGuttersDividerAndTabsFixedWhenScrolled` and
`TestTerminalKeepsUnifiedGutterFixedDuringHorizontalScroll`;
`TestDiffMarkersUseTerminalColorsAndCursorFillsWidth` was retained by name;
`TestSelectionBackgroundKeepsDefaultWeight` moved to
`TestTerminalPreservesCursorAndSelectionStyling`;
`TestSyntaxHighlightingSurvivesDiffStyling` moved to
`TestTerminalHighlightsSyntaxThroughDiffStyling`;
`TestRenderedCodeRowsHaveExactTerminalWidth` moved to
`TestTerminalCodeRowsHaveExactWidth`; and
`TestRenderStyledRowStripsSyntaxBackgroundColors` was retained by name.
