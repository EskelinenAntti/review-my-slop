---
name: signature-refactor
description: Refactor this repository's production Go abstractions until independent Luna reviews pass every signature criterion.
---

# Signature refactor

- Follow [the review guide](../../../docs/signature-review.md). Extract with
  `go run ./scripts/signatures`; keep inventories and reports outside the repo.
- For each function, method, and interface method, request separate reviews of
  responsibility, inputs, outputs, receiver cohesion, file cohesion, package
  cohesion, and meaningful naming. Each criterion scores **1 = pass, 0 = fail**.
- Assign each criterion to a fresh independent Luna agent (`gpt-6-luna`,
  `fork_turns="none"`) with the rubric and complete frozen inventory only.
  Validate matching bytes/digests, complete unique coverage, binary scores,
  and reasons for failures. Reject invalid rounds; never automate judgments
  or steer scores.
- Fix every failure while preserving behavior and checking implementations.
  Exclude tests, test-only helpers, and fixtures from refactoring except for
  production-reference updates. Repeat until all seven criteria pass for every
  signature in a fresh round. Report a blocker if Luna is unavailable.
- Confirm extraction matches the passing round; run `make check`, `make test`,
  and `make build`. Remove generated reports and artifacts; summarize changes
  and validation. Commit and push only when requested; preserve unrelated work.
