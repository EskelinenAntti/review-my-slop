# Overnight readability cleanup

## Goal

Reduce the production AST-size score without changing the program's behavior.

```text
Baseline: 18,519 AST nodes
Command:  make ast-size
```

The score counts every Go AST node in non-test files under `cmd/` and
`internal/`. It deliberately excludes tests and the metric script itself.

## Working rules

1. Keep behavior stable. Do not remove, weaken, or rewrite tests merely to
   make a cleanup easier.
2. Do not change `scripts/ast_size.go` or redefine the metric during the run.
3. Prefer deleting dead code, collapsing duplication, removing needless
   indirection, and making one concept have one clear home.
4. Preserve helpful names. Do not trade understandable intermediate values
   for dense expressions, or create small poorly named wrapper functions to
   move code around.
5. Make focused changes. If a refactor touches multiple concepts, split it
   into independently reviewable commits.

## Loop

1. Record `make ast-size` before a cleanup.
2. Read the relevant code and tests together; identify duplication, obsolete
   abstractions, or unnecessary state and control flow.
3. Make the smallest behavior-preserving simplification.
4. Format touched Go files with `gofmt`.
5. Run `go test -race ./...`, `go vet ./...`, and `make ast-size`.
6. Keep the change only when all checks pass and the score falls, or when it
   establishes a clearly necessary prerequisite for a later reduction.
7. Commit each verified cleanup with its AST-score delta in the commit body.

## Completion report

Report the initial and final AST scores, the absolute and percentage change,
the checks run, and a short list of the concepts simplified. Call out any
remaining high-value cleanup candidates rather than making speculative changes.
