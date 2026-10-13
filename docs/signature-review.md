# Signature responsibility review

Extract production Go function and method signatures with:

```sh
go run ./scripts/signatures -output /tmp/review-my-slop-signatures.txt
```

The extractor includes exported and unexported declarations and explicitly
declared interface methods. Each line includes the relative package directory,
package name, receiver (where applicable), name, parameters, and results.
It excludes `_test.go` files and hidden, vendor, node_modules, testdata, and
testutil directories (including test fixtures and test-only assertion helpers).
Declarations are included regardless of build tags. Bodies and documentation
are omitted. `-root` selects another source tree; omit `-output` for stdout.

For every review round, start an independent Luna agent with fresh context.
Instruct it to run the extractor and read only the resulting signatures, never
source code, implementation, documentation, diffs, or earlier reviews. Score
every line using only the package and signature:

- **3:** The responsibility and intended inputs are clear.
- **2:** The responsibility or inputs are somewhat clear.
- **1:** The responsibility or intended inputs cannot be understood.

Keep the full scored inventory and reasons. Refactor any score-1 production
abstractions to expose a clear responsibility and intuitive inputs; validate
behavior with the existing tests. Repeat extraction and fresh-context review
until every signature scores at least 2, then create a PR. Test functions and
test helpers are excluded from scoring and refactoring scope.

## Recorded reviews

Each complete round used three fresh independent `gpt-6-luna` agents, assigned
non-overlapping batches of at most 100 signatures. Each agent ran the extractor
and read only its assigned output lines. All three extracted inventories were
checked for equality, and their scores were checked for complete line coverage.
Reports retain the canonical extracted signature text alongside the agents'
numeric judgments. Report formatting and signature transcription differences
were normalized against the corresponding inventory without changing scores.

| Round | Signatures | Score 1 | Score 2 | Score 3 | Report |
| --- | ---: | ---: | ---: | ---: | --- |
| 1 | 282 | 19 | 141 | 122 | [Scores](reviews/signatures-round-1.tsv) |
| 2 | 291 | 1 | 17 | 273 | [Scores](reviews/signatures-round-2.tsv) |
| 3 | 288 | 1 | 43 | 244 | [Scores](reviews/signatures-round-3.tsv) |
| 4 | 289 | 0 | 45 | 244 | [Scores](reviews/signatures-round-4.tsv) |

Rounds 1 and 2 excluded `_test.go` files; rounds 3 and 4 additionally excluded
testutil and testdata directories. The inventory also changed as abstractions
and extractor helpers were refactored. No test functions were scored.

The first pass introduced a source-line range abstraction, coupled Git diff
revisions with their original-source reader, moved ANSI color decisions onto
the foreground policy, and clarified escaping, cursor identity, navigation,
matching, and comment viewport responsibilities. Round 2 identified `readIndex`,
which became `readGitIndexFileSource` with explicit repository path parameters.
Round 3 identified the extractor's required Go `main` entry point; its behavior
was moved into `extractProductionSignaturesCLI`, leaving `main` as the launcher.
The final full inventory received no score below 2.
