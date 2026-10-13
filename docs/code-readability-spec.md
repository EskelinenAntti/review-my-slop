# Code readability audit and implementation spec

Baseline: `30a3799` (PR #46). The audit covers every Go package, production
code and tests. Three parallel Luna agents reviewed the application, patch /
editor / comment storage, and patch screen; the coordinating agent reviewed the
CLI, preferences, other screens, key matching, and screen framing.

## Reading contract

Each file should answer one concrete question. Read its main operation first,
then its subordinate operations in the order a reader needs them. Keep a type
and its small supporting methods together. Place a helper with the behavior it
supports, even when that helper has callers in other files. A file's length is
a constraint, not its organizing principle.

Use the vocabulary in `CONTEXT.md`: Patch, Patch range, Comment, and Anchor.
Keep Git's literal diff terminology and the existing `diffscreen` package path.
Framework messages and terminal display coordinates remain technical concepts;
they should not be renamed into business concepts they do not represent.

Names must reveal the result or action. A name such as `render`, `pair`,
`nearest`, or `load` needs context that its call site currently does not supply.
Prefer a precise noun or verb over a new layer of indirection. Do not rename a
clear helper merely to make it longer, introduce generic utility packages, or
create objects whose only purpose is satisfying a lint limit.

Tests should read as examples of the same chapters as production code. Name
fixtures by their structure; keep a single-use fixture with its test. Shared
fixtures must not be owned by an unrelated test file. Retain the existing test
and fuzz entry points and their assertions.

## Findings and required changes

### A — Application orchestration

- **A1:** `navigation.go` mixes layout preferences, search input, and navigation
  bindings. Give these chapters `layout.go`, `search.go`, and `navigation.go`.
  Keep layout persistence and screen resizing together. Navigation should read
  as key dispatch followed by its motion/alignment maps.
- **A2:** `model_update.go` mixes event dispatch, editor completion, comment
  reload, patch refresh, and geometry. Separate event routing (`update.go`),
  patch refresh (`patch_refresh.go`), and comment reload (`comment_reload.go`).
  Put editor completion with the comment workflow and geometry with layout.
- **A3:** `model.go` is a model glossary; `model_init.go` is construction and
  dependency setup. Keep model state distinct from asynchronous event payloads
  where that helps each file's purpose. Rename `keys.go` to `keyboard.go`.
- **A4:** Comment editing currently has several subtly different "finish",
  "begin", and "open" helpers. Order the workflow as mode input, new/edit
  request, editor completion, persistence, and draft reset. Keep source-editor
  targeting in `source.go` and anchor construction in `comment_anchor.go`.
- **A5:** `test_helpers_test.go` mixes model messages, key constructors, rendered
  output, and patch fixtures. Give shared model/key helpers a clear test-harness
  home and patch fixtures their own `patch_fixtures_test.go`. Move `modelPatch`
  out of `model_test.go`; keep comment persistence examples with comment tests.

Required naming direction (update callers and tests together):

| Current name | Intended name / meaning |
| --- | --- |
| `diffView`, `diffOptions` fields | `patchView`, `patchViewOptions` |
| `refreshDiffFunc`, `refreshDiffMsg` | `patchRefreshFunc`, `patchRefreshMsg` |
| `loadRefresh`, `finishDiffRefresh` | `requestPatchRefresh`, `applyPatchRefresh` |
| `loadComments`, `finishCommentsLoad` | `requestCommentReload`, `applyCommentReload` |
| `newWithStore` | `newModelWithStore` |
| `navigate` | `applyNavigationKey` |
| `updateComments` | `handleCommentModeKey` |
| `beginComment`, `finishCommentEdit` | `startNewComment`, `saveCommentDraft` |
| `finishCommentEditor` | `completeCommentEditor` |
| `clearCommentEdit`, `openCommentEditor` | `clearCommentDraft`, `launchCommentEditor` |
| `commentIndex`, `accumulateRange` | `findCommentIndex`, `extendLineRange` |
| `sourceLocation`, `openCurrentLine` | `sourceEditorTarget`, `openFocusedSourceLine` |
| `click`, `drag`, `release`, `wheel` | mouse helpers naming the event/action |
| `testModel`, `requireModel`, `updateModel` | `newTestModel`, `requireAppModel`, `applyTestMessage` |
| `modelPatch`, `coveragePatch`, `longModelPatch` | structural patch fixture names |
| `lineText`, `screenBody` | `focusedLineText`, `renderedScreenBody` |

### B — Patch screen: view, projection, and terminal text

- **B1:** `diff_selection.go` includes selection extraction, source lookup,
  cursor identity ranking, and syntax-line lookup. Separate range extraction,
  cursor lookup/navigation, and cursor restoration. The restoration chapter
  should read as candidate search, ranking, then distance/tie-breaking.
- **B2:** `diff_view.go` includes row model, row construction, change-block
  pairing, and highlighting. Separate projection construction from change-block
  pairing; move syntax-source lookup and highlight attachment into the
  highlighting chapter. Header normalization belongs with row construction.
- **B3:** Private `diffView` and public `View` are easily confused. Name the
  private row model `patchProjection`, its rows `displayRow`, and its constructor
  `newPatchProjection`. Use `patchCursor`, `patchViewport`, `patchRange`, and
  `rowRenderer` for the related private types. Preserve public APIs.
- **B4:** Make the two layers visible in filenames: `view_*.go` for interactive
  view behavior and `projection_*.go` for source-to-row mapping, cursor queries,
  viewport calculations, and row rendering. Keep `highlight.go` and
  `terminal_text.go` as supporting chapters. Do not mechanically split the
  coherent ANSI/color/window helpers into tiny files.
- **B5:** Projection tests currently mix pairing, navigation, selection, and
  cursor restoration (`diff_view_test.go`, `diff_split_test.go`,
  `diff_cursor_test.go`). Regroup by those behaviors; keep view-level behavior
  examples distinct from projection-level examples. Move shared patch builders
  and pointer helpers out of restoration tests. Keep deletion and sticky-header
  fixtures beside their sole consumers.

Required naming direction:

| Current name | Intended name / meaning |
| --- | --- |
| `entry`, `pair`, `render` (highlighter) | `displayRow`, `highlightedSources`, `highlightSource` |
| `first`, `last`, `scan`, `move` | `firstCursor`, `lastCursor`, `scanCursors`, `moveCursor` |
| `valid`, `file`, `line`, `lines` | `validCursor`, `fileAtCursor`, `lineAtCursor`, `selectedLines` |
| `findCursor`, `matchRow` | `restoreCursor`, `matchCursorRow` |
| `identify`, `replaceView` | `identifyCursor`, `replaceProjection` |
| `nearest` | `nearestCursorInViewport` |
| `keepVisible`, `align`, `maxTop` | `keepCursorVisible`, `alignCursor`, `maxViewportTop` |
| `search`, `searchRow`, `searchMetadata` | `searchFromCursor`, `searchDisplayRow`, `searchNonCodeRow` |
| `selected`, `number`, `renderFileRow` | `cursorIsSelected`, `formatLineNumber`, `renderFileHeaderRow` |
| `lineRows`, `sourceText`, `highlightedLine` | row pairing and highlighted-source names describing their results |
| `testPatch`, `longPatch`, `longLinePatch` | structural fixture names describing files, context, or code width |
| `mustFirst`, `renderOne`, `renderTarget` | `mustFirstCursor`, `renderFocusedRow`, `renderViewportAtRow` |

### C — Patch acquisition, editors, and stored comments

- **C1:** Patch acquisition uses generic names for its stages and source
  readers. Name directory loading, default-branch discovery, comparison base,
  tracked-file parsing, and index/revision/worktree source reads explicitly.
  Keep orchestration in `get.go`, Git process access in `git.go`, comparison
  discovery in `comparison.go`, and tracked/untracked chapters distinct.
- **C2:** `tracked.go` mixes file parsing and unified hunk parsing. Keep file
  parsing in `tracked.go` and unified hunk body/header handling in `hunks.go`.
  `text.go` mixes path normalization and terminal-safe text; put path handling
  in `paths.go` and control escaping in `display_text.go`.
- **C3:** `editor.go` mixes Markdown drafts, filesystem lifecycle, and shell
  invocation. Use `draft.go` for draft content/files, `commands.go` for shell
  command construction, and `workflows.go` for the editor lifecycle. Keep the
  process adapter separate if it gives the lifecycle a clearer reading order.
- **C4:** `format.go` mixes consuming pending comments with writing a prompt.
  Use `pending.go` for write-and-acknowledge orchestration and `prompt.go` for
  serialization. Align their tests with those responsibilities.
- **C5:** `store_records.go` mixes bucket lookup/mutation, validation, and JSON
  compatibility. Keep record operations together and move encoding/legacy
  schema into `encoding.go`. Place validation beside record operations or in
  the codec chapter where appropriate, avoiding a tiny utility file. Keep the
  public Store lifecycle in `store.go` and transactions in `store_database.go`.
- **C6:** Shared `testComment` is owned by a store test but used by export tests.
  Move it to shared comment fixtures. Move the added-file path escaping example
  from `get_test.go` to untracked-file tests. Keep other single-use fixtures
  next to their behavior examples.

Required naming direction:

| Current name | Intended name / meaning |
| --- | --- |
| patch `load`, `loadFiles`, `defaultBranch` | `loadPatchFromDirectory`, `loadPatchFiles`, `discoverDefaultBranch` |
| `sourceReader`, `readOld` | `baseSourceReader`, `readBaseSource` |
| `readIndex`, `readRevision`, `readWorkingTree` | names ending in `Source` |
| `parseTracked`, `addedFile`, `gitDiff` | `parseTrackedFiles`, `buildAddedFile`, `runGitDiff` |
| `untrackedPaths` | `splitNULSeparatedPaths` |
| `visibleSource`, `visibleText`, `visibleStrings` | escaping names that distinguish multiline source from inline display text |
| `commentDraft`, `stripUnchangedSuggestion` | `formatCommentDraft`, `removeUnchangedSuggestion` |
| `createCommentFile`, `readCommentFile` | `writeCommentDraftFile`, `readEditedCommentDraft` |
| `commentCommand`, `sourceCommand`, `shellQuote` | `newCommentEditorCommand`, `newSourceEditorCommand`, `quoteShellArgument` |
| `commentWorkflow`, `commandAdapter` | `newCommentEditorWorkflow`, `editorCommandAdapter` |
| `messagesBucket` | `commentsBucket` (preserve its on-disk key) |
| store `transact`, `view`, `open` | `withWriteBucket`, `withReadBucket`, `openDatabase` |
| `pendingBytes`, `findPendingKeys`, `decodeComment` | `pendingCommentBytes`, `findPendingCommentKeys`, `decodeCommentRecord` |
| `appDir` | `applicationDataDirectory` |
| patch test `git`, `writeFile`, `containsKind` | helpers explicitly naming repository commands/files and matching patch lines |

### D — CLI, preferences, and smaller screens

- **D1:** The comment-list screen mixes interaction/scrolling with rendering.
  Keep view state in `screen.go`, rendering in `render.go`; name `startRow` and
  `keepVisible` as `firstVisibleComment` and `keepFocusedCommentVisible`.
  Name `renderRow` as `renderCommentRow`. Group rendering tests separately.
- **D2:** Preference loading/saving and path discovery are coherent but should
  read public Load/Save first, then file reads/writes, then path discovery.
  Prefer `preferencesPath` and local `preferences` to the generic `settings`.
- **D3:** CLI tests mix comment exporting, invalid command dispatch, and shared
  Git setup. Group dispatch and comment-command examples in named files, with
  shared repository setup in a test-fixture file. Keep PTY lifecycle helpers
  beside PTY tests and give the lifecycle assertion an explicit name.
- **D4:** `keymap`, `helpscreen`, `frame`, and `testutil` already have coherent
  file responsibilities. Preserve their simple design; keep test-only assertion
  helpers below the behavior examples when no dependency dictates otherwise.

## Implementation and acceptance

Implement A, B, and C with three parallel Luna agents owning disjoint package
directories. The coordinating agent implements D, integrates their work, and
reviews names and file contents across package boundaries. Finish the spec
before implementation; commit the spec separately so the audit is reviewable.

- Preserve public APIs, user-visible output, key bindings, storage encoding and
  database keys, context cancellation, editor cleanup, and refresh semantics.
- Keep all enabled Revive rules, maximum four parameters, and 200 total lines
  per Go file. No lint exclusions, suppression comments, cache/environment
  workarounds, new dependencies, or unnecessary abstractions.
- Preserve all 164 test/fuzz entry points and their behavior coverage.
- Review the final file-purpose map, helper locations, and names against A–D.
- Run formatting, `make check`, and `make test` (race detection) after integration.
- Update PR #46 with the completed readability work and this audit/spec.

Implementation status: specification complete; implementation pending.
