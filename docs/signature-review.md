# Signature review

Extract the production Go inventory from the repository root:

```sh
go run ./scripts/signatures -output /tmp/review-my-slop-signatures.txt
```

The inventory groups signatures under `file: relative/path.go` headers and
includes package, receiver, name, inputs, and outputs. Explicit interface
methods, including anonymous and local interfaces, are included; bodies and
documentation are omitted. All build tags are included. Use `-root` for another
tree. Tests, `testutil`, `testdata`, hidden directories, `vendor`, and
`node_modules` are excluded. Never exclude unclear production code to pass.

## Independent binary reviews

For every function, method, and explicit interface method, request **seven
separate reviews**, one per criterion. Assign each criterion to a fresh Luna
agent (`gpt-6-luna`, `fork_turns="none"`). Give each reviewer this rubric and the
complete frozen inventory, including file headers, even when assigning only a
subset of signatures. Do not provide source, documentation, diffs, or prior reviews.

Each criterion gets **1 = pass, 0 = fail**; never combine scores.

| Criterion | Pass condition |
| --- | --- |
| Responsibility | An unfamiliar Go developer can infer what concretely happens. |
| Inputs | Parameters clearly communicate what the function needs and their roles. |
| Outputs | Results clearly communicate what the function returns and their roles. |
| Receiver cohesion | The method fits the other functions on its struct or interface. |
| File cohesion | The function fits the responsibilities of other functions in its file. |
| Package cohesion | The function fits the responsibilities of other functions in its package. |
| Meaningful name | The name has a specific, recognizable meaning; familiar words stacked into an invented phrase do not suffice. |

Assess each cohesion level independently. Receiver cohesion passes for
functions without a receiver; absent inputs or outputs pass their respective
criterion. Judge only signatures and grouping: do not invent behavior or demand
unseen type fields, algorithms, or callback internals. Named domain types supply
context. Respect established Go/framework contracts and conventional lifecycle
methods. Do not automate judgments or target a score distribution.

## Validate and refactor

Record each judgment as `line | criterion | score | reason`, using the 1-based
inventory line number and criterion labels above. Reasons are required for 0
and optional for 1. File headers are not scored. Verify reviewer snapshots
byte-for-byte against the canonical inventory and record SHA-256 digests.
Require exactly one judgment per signature per criterion; missing, duplicate,
nonbinary, or unexplained failures invalidate the round.

Fix every failure while preserving behavior. Check names against implementations
and resolve responsibility boundaries and every failing cohesion level. Update
production references in tests as needed; do not refactor tests or test-only
helpers for scoring.

Repeat extraction, fresh reviews, validation, and refactoring until all seven
criteria pass for every signature in one complete round. If Luna is unavailable,
report the blocker without claiming completion. Confirm final extraction matches
the passing inventory, then run `make check`, `make test`, and `make build`.
Remove task-generated inventories, reviews, and build artifacts. Summarize changes
and validation; commit and push the current branch only when requested.
