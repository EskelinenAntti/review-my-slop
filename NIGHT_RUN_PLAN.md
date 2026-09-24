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

1. Keep behavior stable. Tests may be edited when an internal refactor changes
   an implementation detail, but do not remove or weaken behavioral assertions.
   Add focused regression tests whenever they make the preserved behavior
   explicit.
2. Package boundaries may change when that removes needless indirection or
   duplication. Keep public behavior and externally observable APIs stable
   unless the change is strictly internal to this repository.
3. Do not change `scripts/ast_size.go` or redefine the metric during the run.
4. Prefer deleting dead code, collapsing duplication, removing needless
   indirection, and making one concept have one clear home.
5. Preserve helpful names. Do not trade understandable intermediate values
   for dense expressions, or create small poorly named wrapper functions to
   move code around.
6. Make focused changes. If a refactor touches multiple concepts, split it
   into independently reviewable commits.
7. Commit every verified change on the current `chore/ast-size-metric` branch
   only. Do not create, switch to, merge, rebase, or push any branch.

## Loop

1. Record `make ast-size` before a cleanup.
2. Read the relevant code and tests together; identify duplication, obsolete
   abstractions, or unnecessary state and control flow.
3. Make the smallest behavior-preserving simplification.
4. Format touched Go files with `gofmt`.
5. Run `go test -race ./...`, `go vet ./...`, and `make ast-size`.
6. Keep the change only when all checks pass and the score falls, or when it
   establishes a clearly necessary prerequisite for a later reduction.
7. Commit each verified cleanup on `chore/ast-size-metric`, with its AST-score
   delta in the commit body.

## Finish

Continue the loop until the codebase has reached the best behavior-preserving
simplification that can be justified from the code and its tests. Do not stop
after identifying follow-up work: implement it, verify it, and commit it.

At the end, report the initial and final AST scores, the absolute and
percentage change, the checks run, and the concepts simplified.
