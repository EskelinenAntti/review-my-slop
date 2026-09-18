# Rewrite review-my-slop for clarity and explicit modeling

## Goal and boundaries

Rewrite the codebase so its structure, names, and operations explain the product naturally. A reader should be able to follow loading changes, reviewing code, attaching comments, and delivering feedback without reconstructing hidden conventions.

This is a whole-codebase redesign. Existing package boundaries, types, helper functions, and test structure are not the target architecture.

- Keep both diff layouts and the current commands, key bindings, branch comparison, search, editor integration, and comment workflows.
- Existing saved data need not carry forward. Leave the old database untouched; use a fresh database without migration or legacy decoding.
- Keep Go, Bubble Tea, and the existing libraries unless a dependency becomes unnecessary.
- Work through one continuous rewrite without milestone approval gates.
- The existing `go test ./...` baseline passes.

## Package vocabulary and design standard

Use these five internal packages:

| Package | Responsibility |
|---|---|
| `ui` | Terminal interaction, presentation, and event handling |
| `git` | Repository discovery and reading changes |
| `diff` | Changed files, hunks, lines, and operations on them |
| `comments` | Comments, their storage, and delivery |
| `editor` | External editor integration and drafts |

Keep the executable entry point thin. Absorb syntax highlighting into presentation code and directory resolution into the packages that use it. Do not create replacement utility packages.

Package consolidation must not merge unrelated responsibilities. In particular, `diff` owns the meaning of changes; `ui` owns their screen representation. A terminal resize must not alter the identity or meaning of a selected source line.

Naming rules:

- Use familiar nouns and direct verbs. Read names at their call sites, including their package qualifier.
- Prefer short names when unambiguous; do not abbreviate away meaning.
- Avoid vague names such as manager, engine, handler, data, and context when a domain word exists.
- Use concrete types by default. Add small consumer-owned interfaces only at useful substitution boundaries.
- Organize files from the main operation to its supporting details.
- Extract functions to express meaningful concepts, not to meet line-count limits.
- Document decisions and invariants rather than narrating statements.

Follow [Google’s Go guidance on interfaces](https://google.github.io/styleguide/go/best-practices.html). Do not introduce an application framework, generic action bus, or elaborate type hierarchy.

## Mandatory modeling audit: every file

Before and during implementation, inspect every production file and every test file. Build a file inventory in this document; record each file’s responsibility, modeling problems, intended destination, and eventual disposition.

The examples motivating this rewrite—mixed layout and business state, and empty strings selecting behavior—are symptoms. Do not treat fixing those examples as completion.

For every type, field, function, and boundary, ask:

1. What concept does this represent? Identify domain facts, user intent, interaction state, presentation state, and external effects. Separate them where they have different owners or lifetimes.
2. What do its possible values mean? Look for magic strings, sentinel numbers, overloaded zero values, interacting booleans, optional callbacks, and combinations that should never exist.
3. Where are its rules enforced? Find invariants maintained by caller discipline, repeated checks, special initialization order, or knowledge spread across packages.
4. What is authoritative? Find duplicated facts, independently mutable derived values, unstable indices used as identity, and cached information without clear invalidation.
5. What knowledge leaks outward? Identify callers that must understand storage encoding, Git argument conventions, screen coordinates, or another component’s internal bookkeeping.
6. What happens over time? Trace refreshes, edits, cancellation, failures, and out-of-order results. Distinguish request intent from the result currently displayed.
7. Does the abstraction simplify its callers? Remove wrappers that merely forward calls, expose their implementation, or require more concepts than the problem itself.

For each issue:

- State the underlying invariant or distinction.
- Choose the smallest explicit representation that expresses it: a named value, enum, cohesive struct, constructor, or behavior-owning type.
- Make invalid operations difficult to express and reject invalid external input at boundaries.
- Encapsulate unavoidable external conventions inside their adapter.
- Remove the old representation and its compensating checks.
- Test the resulting behavior and invariant.

Do not wrap every primitive or replace every zero value. Ordinary optional values and useful Go zero values are welcome when their meaning is singular and clear.

Audit tests with the same rigor. A passing test does not justify an accidental model. Preserve user-facing intent while replacing assertions that encode internal structure.

## Implementation direction

### Domain and presentation

Keep source identity, line ranges, comment anchors, and comparison intent independent of terminal coordinates and styling.

Create a cohesive presentation abstraction inside `ui` that owns row mapping, cursor movement, viewport, pane selection, and restoration after layout changes. Its callers should request meaningful operations without reconstructing mapping rules.

Unified and side-by-side layouts should share semantic operations while retaining their distinct row-building algorithms. Derive selection text and anchor ranges from one authoritative selection traversal.

Keep raw paths and source content distinct from escaped display text.

### Workflows and effects

Make startup and user workflows read in their natural order. Constructors should establish usable objects; avoid setter sequences and implicit configuration protocols.

Keep Git, filesystem, database, and process effects at explicit boundaries. Commands capture their inputs; completion messages identify the operation they belong to. Define ownership and ordering so stale completions cannot overwrite newer intent or successful edits.

Retain useful existing behavior, including comparison semantics, narrow-terminal fallback, editor handoff, and comment delivery. When an existing behavior is demonstrably erroneous, add a regression test and record the correction rather than preserving it blindly.

### Storage

Retain bbolt and XDG conventions with a fresh `inbox-v2.db`. Remove legacy formats and compatibility branches.

Maintain repository isolation, consistent size limits, short-lived transactions, and safe concurrent use by the UI and CLI.

Deliver comments by writing a snapshot and acknowledging only successfully delivered, unchanged records. Output failures and concurrent edits must not silently discard feedback.

## Tests, execution, and completion

Follow Google’s guidance to [test behaviors rather than implementation details](https://abseil.io/resources/swe-book/html/ch12.html), using the [smallest suitable test](https://abseil.io/resources/swe-book/html/ch11.html).

- Use clear arrange/act/assert tests and descriptive failures.
- Prefer real domain values and temporary repositories/databases. Use small fakes for controlled failures and completion ordering.
- Avoid reflection checks, mock call choreography, test DSLs, arbitrary coverage targets, and large ANSI snapshots.
- Test domain rules independently of terminal layout; test presentation using representative domain values.
- Cover both layouts, selection boundaries, restoration, Unicode and escaping, empty states, failures, concurrent delivery, and stale results.
- Keep a small PTY suite with observable readiness, bounded deadlines, and reliable cleanup.
- Retain parser fuzzing and add focused invariant fuzzing where malformed inputs or navigation sequences create meaningful risk.

Implement continuously in dependency order: establish domain meanings, reshape boundaries, reconnect workflows, replace obsolete code, then review the complete reading path. Keep this document current so another session can resume without rediscovering decisions.

Run focused checks during development. Finish with formatting, `go test -race ./...`, `go vet ./...`, `staticcheck ./...`, a CLI build, and bounded fuzz sessions. Follow repository shell and Go permission instructions.

Completion requires:

- Every original file has an audited disposition.
- Every identified modeling issue is resolved or explicitly justified.
- Domain meaning is independent of layout and external encoding conventions.
- Retained workflows pass behavioral tests.
- No old implementation, temporary adapter, or deferred cleanup remains.
- Documentation explains the architecture and fresh-storage change.
- The final report states changes, intentional behavior corrections, checks actually run, and remaining limitations.

The stopping condition is a coherent, verified implementation—not exhaustion of available runtime.

## Implementation record

This rewrite was completed on `rewrite-review-my-slop` as one continuous change.
The implementation uses these package boundaries:

| Package | Current role |
|---|---|
| `internal/ui` | Command workflows, Bubble Tea model, key handling, diff presentation, viewport, selection, rendering, and syntax highlighting |
| `internal/git` | Repository discovery, Git commands, diff parsing, source loading, untracked-file inspection, and fingerprints |
| `internal/diff` | Raw change-set, file, hunk, and line values plus semantic file identity |
| `internal/comments` | Anchors, comments, bbolt storage, XDG data resolution, prompt formatting, and conditional delivery acknowledgement |
| `internal/editor` | XDG state resolution, private drafts, suggestion handling, and external-editor commands |

The command package is a process-level wrapper that delegates to `internal/ui.Run`. Command parsing, dependency composition, terminal sizing, and both workflows live in packages. `internal/patch`, `internal/gitdiff`, `internal/view`, `internal/tui`, `internal/review`, `internal/inbox`, `internal/highlight`, and `internal/xdg` were removed rather than retained as adapters.

### Modeling decisions

- `diff.ChangeSet` stores repository paths and source text in raw form. Terminal escaping, tab expansion, ANSI styling, row pairing, and viewport coordinates live in `ui`.
- `ui.presentation` is the sole owner of visual rows, cursor movement, pane switching, viewport movement, selection traversal, and restoration. A cursor carries only a screen coordinate and pane; refreshes restore it from file identity, hunk header, line value, and a coordinate proximity hint.
- A single semantic selection traversal produces both quoted comment lines and old/new ranges. Selections cannot cross files or hunks.
- `ui.ViewMode` and `RefreshTarget` represent local versus branch comparison explicitly; no empty string selects behavior.
- Refresh commands carry a monotonically increasing request identity and target. Results from older requests or another comparison mode are ignored. Comment loads carry the comment revision for the same reason.
- `comments.Store` writes only the new `inbox-v2.db` format. It does not decode legacy records. `Snapshot` retains each encoded record and `Acknowledge` deletes it only if the bytes are unchanged after successful output.
- Storage transactions remain short-lived and the store keeps repository isolation, 64 KiB comment limits, a 16 MiB pending limit, and 0700/0600 filesystem permissions.

### Original file inventory and disposition

Every original production and test file was inspected. The following inventory records where its responsibility went:

| Original file or group | Disposition |
|---|---|
| `cmd/review-my-slop/main.go` | Reworked as the thin composition root |
| `cmd/review-my-slop/main_test.go` | Removed; command behavior tests moved to `internal/ui/command_test.go` |
| `cmd/review-my-slop/pty_test.go` | Retained; it remains the bounded startup/quit integration test |
| `internal/patch/types.go` | Removed; modeled by `internal/diff/model.go` |
| `internal/gitdiff/gitdiff.go` | Removed; Git boundary moved to `internal/git/loader.go` with raw values |
| `internal/gitdiff/gitdiff_test.go` | Removed and rebuilt as `internal/git/loader_test.go` |
| `internal/xdg/paths.go` | Removed; XDG resolution moved to `comments` and `editor` consumers |
| `internal/xdg/paths_test.go` | Removed; consumer behavior is covered by comments/editor tests |
| `internal/view/types.go`, `view.go`, `navigation.go`, `selection.go`, `viewport.go`, `render.go` | Removed; consolidated into `internal/ui` presentation files |
| `internal/view/view_test.go`, `behavior_test.go` | Removed and rebuilt as `internal/ui/presentation_test.go` |
| `internal/tui/model.go`, `navigation.go`, `comments.go`, `render.go` | Removed; consolidated into `internal/ui` model, navigation, comments, and render files |
| `internal/tui/model_test.go`, `behavior_test.go` | Removed and rebuilt as `internal/ui/model_test.go` plus presentation tests |
| `internal/review/types.go` | Removed; comments own `Anchor` and `Comment` in `internal/comments/model.go` |
| `internal/inbox/store.go`, `format.go` | Removed; replaced by `internal/comments/store.go` and `prompt.go` |
| `internal/inbox/store_test.go` | Removed and rebuilt as `internal/comments/store_test.go`, including concurrent delivery |
| `internal/highlight/highlight.go` | Removed; highlighting is presentation code in `internal/ui/highlight.go` |
| `internal/highlight/highlight_test.go` | Replaced by raw-display and rendering coverage in `internal/ui/presentation_test.go` |
| `internal/editor/editor.go` | Reworked to consume `comments.Anchor` and own state-directory resolution |
| `internal/editor/editor_test.go` | Added to cover drafts, suggestions, cleanup, and shell quoting |
| `docs/rewrite.md` | Kept as the specification and updated with this implementation audit |

The main intentional behavior corrections are conditional comment acknowledgement, explicit stale-refresh rejection, raw-domain versus escaped-display separation, and the fresh database name. No migration or legacy decoding remains.

### Verification

| Check | Result |
|---|---|
| `go test ./...` | passed |
| `go test -race ./...` | passed |
| `go vet ./...` | passed |
| `go build ./cmd/review-my-slop` | passed |
| `go test ./internal/git -run '^$' -fuzz FuzzParseHunkBody -fuzztime=5s` | passed |
| `go test ./internal/ui -run '^$' -fuzz FuzzPresentationNavigation -fuzztime=5s` | passed |
| `staticcheck ./...` | not run: `staticcheck` is not installed in the environment |
