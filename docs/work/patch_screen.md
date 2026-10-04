# Patch screen work

- Owner: patch_screen
- Paths: `internal/ui/patch`, `docs/work/patch_screen.md`
- Status: implementation complete; dependency integration and validation pending
- Validation: pending lower-level package commits

Behavior coverage migrated or added in `internal/ui/patch/model_test.go`:

- Saved unified/split preference across narrow and wide sizes; toggle persistence and narrow-width rejection (`TestNewUsesSavedSideBySideForWideInitialSize`, `TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize`, `TestSideBySideToggleStillSavesPreference`).
- Header counts, diff rows, footer, and complete-screen appearance (`TestRenderingAndKeyBindingsRemainAvailable`, `TestHeaderShowsAddedAndRemovedLineCounts`).
- Incremental search, wrapping repeat, cancellation, filename matches, and backspace restoration (`TestSearchMovesIncrementallyRepeatsAndRestoresOrigin`, `TestSearchMatchesFileNamesAndBackspaceRestoresOrigin`, `TestSideBySideSearchActivatesPaneAndCancelRestoresIt`).
- Focus/manual refresh, source-editor refresh, default-branch switching, and stale refresh rejection (`TestFocusAndManualRefreshLoadCurrentView`, `TestSourceEditorCompletionRefreshesDiff`, `TestTabTogglesDefaultBranchAndIgnoresStaleRefresh`, plus same-branch generation coverage).
- `CommentSaved` selection cancellation and routing event results.
