# Integration work

- Owner: `/root/integration` (shell/CLI feature)
- Paths: `cmd`, root `internal/ui` shell/settings and legacy UI files/tests
- Status: shell and CLI implementation integrated into `feature/package-split`
- Validation: `/workspace/toolchains/go/bin/go test ./cmd/review-my-slop ./internal/ui` passed

## Legacy UI test migration

The old root UI implementation and test files are removed only after their
behavior assertions are covered by the new owning packages:

- `diff_view_test.go`, `diff_preserve_test.go`, and navigation assertions from
  `diff_behavior_test.go` move to `internal/layout` and `internal/navigation`,
  documented in `docs/work/layout_navigation.md`.
- Rendering and highlighting assertions from `diff_behavior_test.go`, plus
  `highlight_test.go`, move to `internal/render`, documented in
  `docs/work/rendering.md`.
- Patch interaction, search, refresh, editor, and key sequence assertions from
  `tui_behavior_test.go` and `model_test.go` move to `internal/ui/patch`,
  documented in `docs/work/patch_screen.md`.
- Comment editor, suggestion, reload, delete, list scrolling, and footer
  assertions from `tui_behavior_test.go` and `model_test.go` move to
  `internal/ui/comments`, documented in `docs/work/comment_screen.md`.
- Shell help, key alignment, cross-screen comment routing, and bottom footer
  assertions live in `internal/ui/help_test.go` and
  `internal/ui/shell_routing_test.go`.
- `layout_test.go`'s settings round trip moves to
  `cmd/review-my-slop/layout_settings_test.go`.
