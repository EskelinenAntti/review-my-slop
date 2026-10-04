# Package and screen refactor

## Goal

Preserve the existing review loop, keyboard behavior, prompt output, on-disk data,
and terminal appearance while making each package own one kind of logic. The
root `ui` package becomes a screen router instead of implementing Patch browsing
and Comment editing. Remove the forwarding `review` package and the 23-method
`ui.View` interface. Do not replace that interface with another omnibus facade.

## Responsibility and dependency boundaries

| Package | Owns | Must not own |
| --- | --- | --- |
| `patch` | Patch values, content references, Anchors, working-tree source locations | Git execution, display rows, persistence |
| `git` | Repository discovery, default branch, Patch acquisition, parsing and source reads | UI state, feedback |
| `comments` | Comment validation, IDs, timestamps, durable storage, exact acknowledgement | Rendering prompts, terminal interaction |
| `prompt` | Formatting pending Comments for the agent | Storage, acknowledgement |
| `layout` | Unified/split rows and stateless content/cell mapping and search | ANSI styling, mutable navigation |
| `navigation` | Cursor, active Patch range, viewport, movement and preservation | Rendering, keys, storage |
| `render` | Highlighting, terminal styling, clipping, gutters and sticky headers | Mutating navigation, key handling |
| `editor` | External editor command construction, temporary text editing resources | Suggestions, Comment storage, Bubble Tea |
| `ui/patch` | Patch screen keys, search input, branch/layout switching, refresh ordering, source editing | Comment persistence and editing, rendering algorithms |
| `ui/comments` | Comment screen keys, list state, reload ordering, draft/suggestion lifecycle, saves and deletes | Git, Patch navigation |
| `ui` | Screen routing, help and application quit, forwarding terminal events | Cursor/range state, drafts, refresh revisions, Git or DB setup |
| `cmd/review-my-slop` | Dependency construction and the two command workflows | Screen implementation |

There must be no cycles. Screens may consume the lower-level packages. `render`
may consume `layout` and an immutable `navigation.Snapshot`; it never updates a
Navigation. `patch` imports no project packages. `layout` imports `patch` only.
Existing ANSI and Chroma libraries remain dependencies rather than reimplemented
utility packages. There is no new `app` or generic workflow package.

## Frozen public contracts

All paths below are under `internal`. Existing Patch/File/Hunk/Line values remain
source compatible. Signatures in this section are the handshake between workers;
changes require notifying the integrator and affected owners before using them.

### patch

Move the existing Anchor value (including JSON tags) from comments to patch.
`comments.Anchor` may remain a type alias for storage compatibility, not a second
definition. Add:

```go
type Position struct { File, Hunk, Line int }
type Range struct { File, Hunk int; Lines []int }
type Location struct { Path string; Line int }
func (p Patch) Anchor(r Range) (Anchor, error)
func (p Patch) SourceLocation(pos Position) (Location, error)
func (p Patch) Counts() (added, removed int)
```

Validate every index; an empty range is an error. Anchor deduplicates selected
line indices, preserves Patch order, includes only selected lines and computes
both old/new ranges. SourceLocation preserves current path/line fallback rules.
Patch range membership is resolved in layout; patch never receives screen rows.

### git, comments, prompt

Move loader.go and its tests from patch to git. Keep Loader/Runner/ExecRunner in
git for the existing meaningful fake-runner tests. Add a bound repository:

```go
func Open(ctx context.Context, directory string) (*Repository, error)
func (r *Repository) Root() string
func (r *Repository) Load(ctx context.Context, base string) (patch.Patch, error)
func (r *Repository) DefaultBranch(ctx context.Context) (string, error)
```

Keep existing comments.Store methods for storage tests and introduce a bound
Queue adapter for screen/CLI use. This avoids changing the database format.

```go
type Draft struct { ID string; Anchor patch.Anchor; Body string }
func Bind(store Store, repository string) Queue
func (q Queue) List() ([]Comment, error)
func (q Queue) Save(draft Draft) (Comment, error)
func (q Queue) Delete(id string) error
func (q Queue) Acknowledge(snapshot []Comment) error
// package prompt
func Write(w io.Writer, items []comments.Comment) error
```

Save edits the existing Comment by ID without losing creation metadata; missing
IDs must fail. Empty-body interpretation (cancel new/delete edited) belongs to
the editing screen; storage continues rejecting empty bodies. Acknowledge removes
only the exported snapshot. CLI delivery is List -> prompt.Write -> Acknowledge.
Writer failure keeps feedback. Preserve byte-for-byte current prompt output and
legacy records/permissions/size limits. Remove comments.WritePrompt after callers
and tests move to prompt.

### layout

```go
type Format uint8 // Unified, Split
type Pane uint8 // Left, Right; Other() Pane
type Direction int8 // Backward=-1, Forward=1
type Cell struct { Row int; Pane Pane }
type Selection struct { First, Last Cell }
type RowKind uint8 // FileRow, MetadataRow, HunkRow, LineRow
type Row struct {
    Kind RowKind
    File, Hunk int
    LeftLine, RightLine int // -1 denotes no line
    Text string // plain header or metadata, not ANSI code
}
func Build(p patch.Patch, format Format) *Document
func (d *Document) Patch() patch.Patch
func (d *Document) Format() Format
func (d *Document) RowCount() int
func (d *Document) Row(index int) Row
func (d *Document) Valid(cell Cell) bool
func (d *Document) Position(cell Cell) (patch.Position, bool)
func (d *Document) Locate(pos patch.Position, pane Pane) (Cell, bool)
func (d *Document) Range(selection Selection) (patch.Range, error)
func (d *Document) Find(text string, from Cell, direction Direction) (Cell, bool)
```

Document is immutable by convention; no method mutates its Patch or rows. Range
rejects cross-file/hunk selections and preserves active-pane semantics, including
mixed endpoints on one paired row. Build does not highlight. Search preserves
case-insensitive/wrapping/file-name behavior but does not move navigation.

### navigation and render

```go
type Alignment uint8 // Top, Middle, Bottom
type Viewport struct { Top, LeftColumn, Width, Height int }
type Snapshot struct {
    Cursor *layout.Cell
    Selection *layout.Selection
    Viewport Viewport
}
func New(d *layout.Document, width, height int) *Navigation
func (n *Navigation) Move(direction layout.Direction)
func (n *Navigation) First()
func (n *Navigation) Last()
func (n *Navigation) Jump(cell layout.Cell)
func (n *Navigation) JumpFile(direction layout.Direction)
func (n *Navigation) SwitchPane(pane layout.Pane)
func (n *Navigation) BeginSelection()
func (n *Navigation) CancelSelection()
func (n *Navigation) ScrollHorizontal(columns int)
func (n *Navigation) HalfPage(direction layout.Direction)
func (n *Navigation) Align(alignment Alignment)
func (n *Navigation) Resize(width, height int)
func (n *Navigation) Replace(d *layout.Document)
func (n *Navigation) Snapshot() Snapshot
func (n *Navigation) Progress() int
// package render
type Theme struct { Dark bool }
func Terminal(d *layout.Document, state navigation.Snapshot, theme Theme) string
```

Snapshot must be detached (copy cursor/selection values). Empty documents have a
nil cursor. Navigation owns sticky-header visibility accounting and horizontal
limits based on plain source widths; renderer owns painting the sticky header.
Preserve original selection constraints, empty-pane skipping, half-page visual
rows and replacement/layout position fallback. Reject selection-invalid movement
atomically. Rendering owns syntax coloring and its cache if useful; geometry must
never depend on ANSI bytes. Keep exact terminal widths and sticky header behavior.

### editor

```go
func Command(command, path string, line int) *exec.Cmd
func Prepare(command, text string) (*Edit, error)
func (e *Edit) Command() *exec.Cmd
func (e *Edit) Finish(processErr error) (string, error)
func (e *Edit) Close() error
```

Command uses the current shell-quoting behavior; line zero omits the +line flag.
Prepare creates a secure temporary Markdown file. Finish reads and removes it;
process failure also removes it. Close is idempotent. Screens use tea.ExecProcess
on the returned command. Comment suggestions stay in ui/comments.

### screen and shell contracts

Both screen packages have concrete pointer Models (not another View interface):

```go
func New(initial Initial, dependencies Dependencies) *Model
func (m *Model) Update(msg tea.Msg) tea.Cmd
func (m *Model) Resize(width, height int)
func (m *Model) Render() string // complete header/body/footer, same appearance
```

ui/patch:

```go
type Initial struct {
    Patch patch.Patch
    DefaultBranch string
    SideBySide bool
    Width, Height int
}
type Dependencies struct {
    Load func(base string) (patch.Patch, error)
    SaveLayout func(bool) error
}
type CommentRequested struct { Anchor patch.Anchor }
type CommentsRequested struct{}
type HelpRequested struct{}
type QuitRequested struct{}
type CommentSaved struct{}
type Failure struct { Err error }
```

Key handlers emit routing events as tea.Cmd results. The Patch screen owns source
editor execution through editor.Command and refresh after its completion. Focus
and R reload; tab toggles the default comparison; reject obsolete branch AND older
same-branch refresh requests. Theme changes rebuild styling without disturbing
logical position. CommentSaved cancels selection. Errors remain on the screen.

ui/comments:

```go
type Initial struct { Items []comments.Comment; Width, Height int }
type Dependencies struct {
    List func() ([]comments.Comment, error)
    Save func(comments.Draft) (comments.Comment, error)
    Delete func(id string) error
}
type Show struct{}
type BackRequested struct{}
type QuitRequested struct{}
type Saved struct { FromPatch bool }
type Cancelled struct { FromPatch bool }
type Failed struct { Err error }
func (m *Model) Begin(anchor patch.Anchor) tea.Cmd
```

Show loads pending Comments. Begin starts creation from Patch browsing, without
changing the active screen; its completion is routed back to this Model. It owns
editor draft state and must capture immutable IDs/anchors in asynchronous work,
not row indices. Editing empty text deletes an existing Comment; empty new text
is discarded. Maintain suggestion fence rules and unchanged-suggestion stripping.
Reload results are rejected after mutations and after newer reloads. Errors retain
current items. q/Esc/C return to Patch; Ctrl-C quits. A successful FromPatch save
emits Saved and the shell forwards CommentSaved to the Patch screen.

The shell constructs both children in ui.New(initial Initial, deps Dependencies)
and implements tea.Model. Shell Initial contains Patch, Comments, DefaultBranch,
SideBySide and Size; Dependencies contains Load, List, Save, Delete and SaveLayout
callbacks with the exact types above. No legacy setters or 23-method View remain.
Keep Size and DefaultSize. Move settings IO to cmd, so ui.New performs no disk IO.
Route background-color/resize/focus events as appropriate, child async completions
to their owning child even when inactive, and ordinary keys to the active child.
Help is static shell content; no package needed. Initialize background detection
as today. cmd opens dependencies and loads initial data/preferences. Delete review.

## Preserved behavior and validation

- Default/code/comments commands and usage errors; CLI prompt consumes only the
  current repository's exported Comments and never consumes on writer failure.
- Database path/permissions, legacy decoding, limits and exact acknowledgement.
- Vim sequences, pending-key consumption, search cancellation/repeat, pane search,
  Comment suggestions, empty-edit semantics and source-file editor invocation.
- Unified/split pairing, active-pane ranges, sticky headers, ANSI widths/colors,
  horizontal limits and syntax highlighting.
- Refresh fallback and selection preservation across layout/terminal thresholds;
  branch and same-branch obsolete-result rejection; mutation/reload ordering.
- Startup size and PTY nonblank first frame.

Move existing tests to their owning packages and adapt them to the real new API.
Retain behavioral assertions; do not delete coverage merely because private field
names changed. No old production implementation or compatibility facade may remain
to make tests pass. Tests may use small assembly helpers. Add only meaningful tests
for the new concurrency guards, invalid references and resource cleanup.

Run go test ./..., go test -race ./..., go vet ./..., and go build ./cmd/review-my-slop.
Use /workspace/toolchains/go/bin/go: /usr/bin/go is a different program. Respect
AGENTS.md: one shell command at a time, no Go cache/env overrides; retry an actual
sandbox-blocked Go command with require_escalated. Standard cache/modules need
escalated access in this environment. Do not alter go.mod dependencies unnecessarily.

## Parallel work ownership

Workers use separate branches/worktrees derived from the spec commit. Each creates
docs/work/<task>.md stating owner, paths, status and validation before implementation
and updates it in the final commit. Send claim/final commit SHAs to the integrator.
Only the integrator writes feature/package-split and cherry-picks worker commits.
Workers must not cherry-pick into or edit the integration worktree themselves.

| Task | Exclusive paths | Inputs |
| --- | --- | --- |
| core | internal/patch, internal/git, internal/comments, internal/prompt, internal/review removal | Frozen content/storage contracts |
| layout_navigation | internal/layout, internal/navigation | patch value/reference contracts |
| rendering | internal/render | layout rows and navigation snapshots |
| patch_screen | internal/ui/patch | layout/navigation/render/editor contracts |
| comment_screen | internal/ui/comments, internal/editor | comments/patch contracts |
| integration | cmd, remaining internal/ui root, old ui tests/files removal, docs final status | All worker commits |

Each worker copies/adapts relevant behavior tests into its own packages without
editing old root ui tests. Shell/CLI owner removes old source/tests after
confirming coverage migration. Claim records are disjoint files. Each feature
worker integrates its own claim and implementation commits onto the shared
feature branch while holding `/tmp/review-my-slop-integration.lock`. The
shell/CLI owner removes old source/tests after confirming coverage migration.
The root task owner handles combined branch fixes and final checks. Any contract
change is coordinated explicitly. Workers can read each other's worktrees but
never edit them.

## Acceptance

The feature branch builds and passes the checks; the original interaction/output
contract remains intact; git/persistence/rendering are absent from the root UI
implementation; navigation cannot render; rendering cannot mutate navigation;
and root ui owns no cursor, Patch range, draft, or refresh-generation state.
