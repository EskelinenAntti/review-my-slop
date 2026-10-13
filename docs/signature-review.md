# Signature clarity and cohesion review

Extract production Go function and method signatures with:

```sh
go run ./scripts/signatures -output /tmp/review-my-slop-signatures.txt
```

Each source file starts with a `file: relative/path.go` header before its
signatures. Signature lines include the relative package directory, package
name, receiver where applicable, name, parameters, and results. Explicit
interface methods, including anonymous and local interfaces, are included.
Anonymous interfaces include their source location. Bodies and documentation
are omitted. Declarations are included regardless of build tags. Use `-root`
for another source tree; omit `-output` for stdout.

Go test files, test-only helpers in `testutil`, and fixtures in `testdata` are
excluded, as are hidden directories, `vendor`, and `node_modules`. Do not
exclude unclear production code to improve scores.

Each round uses independent Luna agents with fresh contexts. Reviewers may run
the extractor and read only the complete extracted inventory. They must not
inspect source files, implementations, documentation, diffs, or prior reviews.
Each reviewer must see all signatures to assess package and cross-file
cohesion, even when assigned a non-overlapping subset to score.

Give every function and explicit interface method two independent 1-to-3 scores:

- **Clarity:** Assess responsibility, inputs, and outputs together as one score.
  **3** means all three can be inferred clearly; **2** means at least one is
  only somewhat clear; **1** means at least one cannot be inferred.
- **Cohesion:** Assess how the function fits the other functions of its receiver
  (struct or interface), source file, and package. **3** means a coherent fit at
  every applicable level; **2** means a questionable fit at any level; **1**
  means a clear responsibility conflict at any level. Use the lowest applicable
  level score, so a strong receiver fit cannot hide a file or package violation.

For cohesion below 3, include one or more predefined short codes identifying
all affected levels: `S` (struct/receiver/interface), `F` (file), `P` (package).
Include a short specific reason for each code and for clarity below 3. When
both scores are 3, the two scores alone suffice. Never invent implementation
behavior; judge only what the signatures and grouping reveal. Uncertainty
about internals alone is not a violation. Clarity concerns the role of the
function and its inputs and outputs, not exact algorithms, boundary conditions,
or callback implementations. A named domain type supplies domain context;
reviewers must not require its unseen fields. Respect established Go/framework
contracts, including interface names and conventional lifecycle methods.
Do not use automated name-pattern scoring or a desired score distribution.

Record one judgment per signature, identified by its 1-based inventory line
number, as `line | clarity | cohesion | codes and reasons` (omit the final
field when both scores are 3). File headers are context, not scored entries.
Compare each reviewer snapshot byte-for-byte with the canonical inventory,
record its SHA-256 digest, and validate complete, unique coverage and scores in
range. Missing, duplicate, invalid, or unexplained judgments invalidate a round.

Refactor every production abstraction with either score below 3, preserving
behavior. Address every cited receiver, file, and package violation. Prefer
coherent responsibilities and explicit input/output types to shallow helpers.
Existing tests validate changes; update production references in tests as
needed, but do not rename test functions or test-only helpers or refactor them
for scoring purposes.

Repeat extraction, fresh Luna review, validation, and refactoring until every
signature has both scores at 3 in a complete round. Confirm the final extracted
bytes still match that round, then run `make check`, `make test`, and `make build`.
Remove intermediate inventories, snapshots, judgments, reports, and build
artifacts after validation. Do not retain generated score lists or archives in
the repository. Commit the final changes and push the current remote branch
when requested.
