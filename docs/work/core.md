# Core package split work

- Owner: core worker
- Paths: `internal/patch`, `internal/git`, `internal/comments`, `internal/prompt`, `internal/review` removal, `docs/work/core.md`
- Status: implementation complete; integrated into feature/package-split as 7eb9c2f
- Validation: `/workspace/toolchains/go/bin/go test ./internal/patch ./internal/git ./internal/comments ./internal/prompt` passed.
