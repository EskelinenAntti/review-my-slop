# Non-test Go reduction audit

Baseline: `54f9db9` (main when work began). Branch: `refactor/reduce-non-test-loc`.

Formatted non-test Go physical lines: **3,641 → 3,416**, removing **225 lines (6.18%)**. Tests are excluded. Excluding blank lines and standalone `//` comments: **3,250 → 3,028 (6.83%)**. This secondary count is a textual check, not a full Go lexer metric.

The 50% target is **not achieved**. A 50% physical-line result would require at most 1,820 lines, leaving 1,596 more lines to remove. Multiple passes covered every original non-test Go file. The final pass found no further changes that I could justify as both smaller after gofmt and consistent with the behavior, readability, and frozen public-declaration constraints. This is an audit of the investigated candidates, not a proof that no other refactor could ever save lines.

Package documentation was retained verbatim and moved beside entry points. No production functionality was moved into tests or generated code. Existing public function signatures, complete public type declarations, constants, and variables were checked against the baseline with Go's AST parser.

## File-by-file decisions

| Original file | Before | After | Removed | Changes and remaining constraints |
| --- | ---: | ---: | ---: | --- |
| `cmd/review-my-slop/main.go` | 60 | 48 | +12 | Shared CLI validation and store/Patch setup. Keep distinct usage errors and validate before accessing storage or Git. |
| `internal/app/comments.go` | 209 | 198 | +11 | Shared editor-failure cleanup, deferred completed-edit cleanup, removed redundant list checks. Keep save/delete failure semantics, revisions, anchoring, and working-tree location checks. |
| `internal/app/doc.go` | 2 | 0 | +2 | Moved package documentation verbatim into run.go; removed its extra package declaration. |
| `internal/app/model.go` | 400 | 379 | +21 | Replaced repeated motion cases with a binding table; removed trivial private setters and single-use function types. Keep distinct message guards, stale-load revisions, mouse actions, and meaningful save callback type. |
| `internal/app/navigation.go` | 40 | 39 | +1 | Removed intermediate toggle variable. Keep preference-save errors and screen reconfiguration together. |
| `internal/app/render.go` | 53 | 51 | +2 | Passed the help bindings directly. Keep all existing help text and separate screen routing. |
| `internal/app/run.go` | 50 | 48 | +2 | Shared terminal-size fallback loop; removed a trivial setter. Now also holds package documentation. Keep store/setting failure order and injected terminal/program hooks. |
| `internal/comments/doc.go` | 4 | 0 | +4 | Moved package documentation verbatim into store.go. |
| `internal/comments/format.go` | 78 | 78 | +0 | Retained: batching output changes individual writer calls, partial-output failure boundaries, and interactions with custom writers. Sticky-error/iterator abstractions add machinery or continue traversal after failure. |
| `internal/comments/paths.go` | 24 | 20 | +4 | Inlined a one-use configurable path helper. Keep absolute-XDG and home fallback behavior. |
| `internal/comments/store.go` | 272 | 256 | +16 | Shared exact comment lookup, removed unused bucket-name abstraction, simplified acknowledgment membership and default-store return. Keep legacy scanning, transaction rollback, pending-byte limits, permissions, and legacy-specific JSON validation. |
| `internal/comments/types.go` | 20 | 20 | +0 | Retained public declarations exactly, including JSON tags; protected by the requested API constraint. |
| `internal/editor/doc.go` | 3 | 0 | +3 | Moved package documentation verbatim into workflows.go. |
| `internal/editor/editor.go` | 126 | 108 | +18 | Shared suggestion construction and simplified backtick-run detection. Keep edited-suggestion preservation, shell quoting, draft permissions, and distinct create/write/close/read errors. |
| `internal/editor/workflows.go` | 75 | 77 | -2 | Now holds package documentation. Retained workflow code: drafts must be created after terminal handoff, deleted afterward, and commands must receive supplied streams. General setup callbacks add indirection for little or no formatted-line saving. |
| `internal/patch/doc.go` | 4 | 0 | +4 | Moved package documentation verbatim into get.go. |
| `internal/patch/get.go` | 402 | 384 | +18 | Shared index/revision reads, common omitted-file initialization, and short snapshot/hunk literals. Keep Git options, cancellation precedence, branch discovery, symlink/binary/size handling, sanitization, and complete source snapshots. |
| `internal/patch/types.go` | 54 | 54 | +0 | Retained public declarations and their documentation exactly; protected by the requested API constraint. |
| `internal/settings/doc.go` | 2 | 0 | +2 | Moved package documentation verbatim into settings.go. |
| `internal/settings/settings.go` | 87 | 87 | +0 | Shared temporary-file close cleanup; now holds package documentation. Keep explicit permissions (including umask/existing-path behavior), close errors, atomic replacement, and read/decode errors. |
| `internal/ui/commentscreen/screen.go` | 126 | 123 | +3 | Used standard slice lookup and combined bounds updates. Keep ID-preserving focus, independent item copying, click geometry, Unicode width, and clipping. |
| `internal/ui/diffscreen/diff_navigation.go` | 154 | 139 | +15 | Simplified cyclic search, file traversal, and forward pane fallback; reused existing Direction. Keep pane priority, same-file upward fallback, cross-file downward fallback, and metadata search behavior. |
| `internal/ui/diffscreen/diff_preserve.go` | 74 | 71 | +3 | Removed a validity check already enforced by findCursor. Keep source identity, hunk matching, selection translation, and relative viewport restoration. |
| `internal/ui/diffscreen/diff_render.go` | 244 | 228 | +16 | Shared line prefixes and row styling; simplified range bounds, padding, and reset handling. Keep fixed gutters, asymmetric scrolling, empty panes, ANSI color filtering, and exact widths. |
| `internal/ui/diffscreen/diff_selection.go` | 124 | 117 | +7 | Simplified ordered bounds and absolute distance. Keep same-hunk restrictions, cross-pane single-row handling, source ranking, and tie order. |
| `internal/ui/diffscreen/diff_types.go` | 47 | 31 | +16 | Removed duplicate private Direction/Alignment concepts; retained cursor/pane/viewport/selection representations. Arithmetic pane toggling would obscure behavior for unsupported pane values. |
| `internal/ui/diffscreen/diff_view.go` | 119 | 114 | +5 | Removed a one-use highlighting wrapper and grouped paired indices. Keep adjacent change-block pairing and highlighting each side using its own path. A single-pass pairing state machine requires less obvious bookkeeping. |
| `internal/ui/diffscreen/diff_viewport.go` | 167 | 163 | +4 | Simplified visibility bounds and reused Alignment/Direction. Keep overflow-safe scrolling, sticky-header geometry, pane fallback priority, and terminal-cell widths. |
| `internal/ui/diffscreen/highlight.go` | 33 | 28 | +5 | Removed unnecessary io.ReadAll on a bytes.Buffer and shared final line splitting. Keep empty-source, syntax-error fallback, themes, and trailing-newline handling. |
| `internal/ui/diffscreen/mouse.go` | 65 | 59 | +6 | Combined related rejection checks. Keep drag cancellation, original-hunk/pane restrictions, sticky-header adjustment, divider rejection, and zero-distance behavior. |
| `internal/ui/diffscreen/screen.go` | 414 | 389 | +25 | Shared motion dispatch and successful cursor-result handling; simplified search state, selection, and count handling. Keep public declarations, search origin/accepted term, selection extension, layout restoration, and footer precedence. |
| `internal/ui/helpscreen/screen.go` | 35 | 35 | +0 | Retained: both loops serve distinct purposes (measure terminal-cell width, then render aligned/clipped lines). Byte-based formatting changes Unicode/ANSI alignment. |
| `internal/ui/internal/frame/frame.go` | 25 | 23 | +2 | Simplified header/body assembly. Keep at least one body row, precise clipping, final blank line, and cell hit-testing. |
| `internal/ui/keymap/keymap.go` | 49 | 49 | +0 | Retained: slices.Contains replaces two loop lines but adds two import/spacing lines, so produces no formatted physical-line reduction. Keep independent pending state, copied key slices, prefix retries, and consumed continuations. |

## Behavior verification

- Baseline tests passed before changes.
- A temporary deterministic public-API replay ran 100 Patch scenarios with 200 UI operations each, using seed `832714`. Operations covered movement, alignment, scrolling, selection, search, invalid motions/directions, resize, layout/theme changes, Patch replacement, and mouse dragging.
- For each step, the replay hashed full rendered output and public `Current`/`Selected` results. Both baseline and final branch produced SHA-256 `978ddc0bcf68bded325804f22eb9d9f09ef02afadb9b3ae677e5bceb9ad5c309`. The replay was removed from both checkouts after comparison; it is not part of the LOC measurement or committed tests.
- Existing integration tests and added regression tests cover legacy storage-key updates/deletion, corruption before a target record, empty/deletion-only suggestions, zero-store path errors, unexpected line kinds, and renamed-file highlighting through the actual row projection.
- The pre-existing staticcheck unused test assignment was corrected without removing either preference-toggle operation or its assertion.
- Final validation passed: `go test -race ./...`, `go vet ./...`, `staticcheck` v0.8.1, CLI build, public-declaration AST comparison, and `git diff --check`.
