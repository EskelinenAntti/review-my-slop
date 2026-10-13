# Repository Instructions

- Run one shell command at a time. Do not chain commands with `&&`, `||`, `;`, pipes, or similar shell operators.
- Do not override `GOCACHE` or other Go environment settings to work around sandbox or permission failures.
- If a standard Go command is blocked by sandbox permissions, immediately retry the same command with an approval request instead of changing the command or Go environment.

## Signature responsibility reviews

- Exclude test files, test-only helpers, and test fixtures from extraction,
  scoring, and abstraction refactoring. Existing tests still validate changes.
- Use `go run ./scripts/signatures` to extract production signatures.
- Each scoring round must use independent Luna agents with fresh context.
  Reviewers may run the extractor and read its output, but must not inspect
  implementations, source files, documentation, diffs, or prior reviews.
- Refactor score-1 and score-2 production abstractions and repeat until every
  signature scores 3. Do not rename test functions or test-only helpers. See `docs/signature-review.md` for the rubric and workflow.
