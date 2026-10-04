# Layout and navigation work

- Owner: layout/navigation worker
- Paths: `internal/layout`, `internal/navigation`, this status file
- Status: implementation complete; targeted behavior tests pass
- Validation: `/workspace/toolchains/go/bin/go test ./internal/layout ./internal/navigation`
- Migrated behavior: `diff_view` unified/split movement, search, file jumps, ranges,
  unequal block pairing, empty pane handling, and position fallbacks; `diff_behavior`
  pane switching, visual-row movement, sticky-header viewport accounting,
  progress, and horizontal limits; `diff_preserve` cursor, selection, and viewport
  preservation across layout changes and empty replacement. Renderer-only width,
  ANSI, and styling assertions remain with the render owner.
- Claim commit: pending
- Final commit: pending
