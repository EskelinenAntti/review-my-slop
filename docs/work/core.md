# Core package split work

- Owner: core worker
- Paths: `internal/patch`, `internal/git`, `internal/comments`, `internal/prompt`, `internal/review` removal, `docs/work/core.md`
- Status: claimed; implementation in progress
- Validation: baseline `go test ./...` reported passed by task coordinator; package-specific checks to follow.
