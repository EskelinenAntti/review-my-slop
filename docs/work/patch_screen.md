# Patch screen work

- Owner: patch_screen
- Paths: `internal/ui/patch`, `docs/work/patch_screen.md`
- Status: implementation complete; dependency integration and validation pending
- Validation: pending lower-level package commits

The screen exports `Initial`, `Dependencies`, and the frozen routing events. It exposes `Failure{Err error}` for shell-translated Patch-originated failures. It owns branch and same-branch refresh generations, source editor effects, selection-to-anchor requests, keyboard/search state, and complete Patch rendering.

Coverage migrated or added in `internal/ui/patch/model_test.go`:

- Wide/narrow split preference, resize threshold, cursor screen-row preservation, layout save/reject behavior, and header/footer appearance.
- `G`/`gg`, consumed pending keys, `z` alignment, Ctrl-W panes, `[f`/`]f`, horizontal step/end/reset, and selection hunk limits.
- Incremental search, wrapping repeat, cancel/backspace restoration, filename search, and split-pane search behavior through navigation.
- Focus/R/source-editor refresh, empty-diff fallback, semantic cursor fallback, branch changes, older same-branch result rejection, and obsolete-branch rejection.
- Progress labels, source editor availability/completion, selected-line Anchor mapping, routing events, `CommentSaved` selection cancellation, and surfaced asynchronous failures.

The shell still owns the static Help screen and terminal view setup; Patch key routing is covered through emitted `HelpRequested`.
