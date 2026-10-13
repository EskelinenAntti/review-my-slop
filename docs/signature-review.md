# Signature responsibility review

Extract production Go function and method signatures with:

```sh
go run ./scripts/signatures -output /tmp/review-my-slop-signatures.txt
```

The extractor includes exported and unexported declarations and explicit
interface methods, including anonymous and local interfaces. Each line includes
the relative package directory, package name, receiver where applicable, name,
parameters, and results. Anonymous interfaces also include their source location.
Bodies and documentation are omitted. Declarations are included regardless of
build tags. Use `-root` for another source tree; omit `-output` for stdout.

Go test files, test-only helpers in `testutil`, and fixtures in `testdata` are
excluded. Hidden directories and dependency directories (`vendor` and
`node_modules`) are also excluded. Do not exclude unclear production code to
improve review results.

Each round starts independent Luna agents with fresh contexts. Reviewers run
the extractor and read only their assigned signature lines. They must not read
source code, implementations, documentation, diffs, or earlier assessments.
Large inventories may use non-overlapping batches. Each signature is judged
individually using only its package, receiver, name, parameter names and types,
and result types:

- **3:** The responsibility and intended inputs are clear.
- **2:** The responsibility or intended inputs are only somewhat clear.
- **1:** The responsibility or intended inputs cannot be inferred.

Do not use automated name-pattern scoring or a desired score distribution.
Respect established Go and framework contracts. Preserve required names and
contracts and improve the surrounding abstractions where needed.

Record each signature and judgment, with a specific reason for each judgment
below 3. Compare every reviewer's extracted bytes with the canonical inventory,
record its SHA-256 digest, and check that each line has exactly one judgment.
Duplicate, omitted, invalid, or unexplained judgments invalidate a round.

Refactor every production abstraction below 3, preserving behavior. Prefer
coherent responsibilities and explicit input types to shallow helpers. Existing
tests validate each change; update their production references as needed without
changing tests solely to improve review results. Test names are never extracted
or scored.

Repeat extraction, fresh review, validation, and refactoring until every
production signature receives 3 in a complete round. Confirm the final extracted
bytes still match that round, then run `make check`, `make test`, and `make build`.

Retain the full review evidence in a separate archive before removing loose
inventories and score reports from the working tree. The archive preserves
historical reports, each new canonical snapshot, reviewer snapshots, judgments,
and consolidated reports with snapshot digests. Deliver that archive alongside
the pull request; do not leave generated name or score lists in the repository.
