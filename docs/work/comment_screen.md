Owner: comment_screen
Paths: internal/ui/comments, internal/editor, docs/work/comment_screen.md
Status: implementation complete; ready for feature-branch integration
Validation: `/workspace/toolchains/go/bin/go test ./internal/editor ./internal/ui/comments` passed (standard Go build cache required escalation).

The comments screen owns list navigation, editor sessions, ID-based saves and deletes, pending-ID mutation reservations, and reload generation checks. It emits `Saved{FromPatch:true}` after a successful Patch-originated save, `Cancelled{FromPatch:true}` for a blank new composition so the shell can cancel Patch selection, and `Failed{Err:...}` for Patch-originated editor or persistence errors. The shell maps those events to `ui/patch.CommentSaved` and `ui/patch.Failure`.

Migrated behavior coverage from `internal/ui/tui_behavior_test.go`:

- External editor invocation, secure temporary Markdown files, and cleanup on completion or process failure (`internal/editor/editor_test.go`).
- Suggestion fence sizing, stripping unchanged suggestions, and retaining edited suggestions (`internal/ui/comments/model_test.go`).
- Captured anchor immutability, blank new comment discard/selection cancellation, and empty edited comment deletion by ID.
- Editor and storage failure routing, same-ID overlapping mutation prevention, and stale reload rejection after newer reloads or mutations.
- Existing comments screen appearance/listing/edit/delete behavior remains in the same renderer structure; shell integration owns full terminal/PTY checks.
