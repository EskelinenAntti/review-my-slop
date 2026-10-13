---
name: signature-refactor
description: Refactor this repository's production Go abstractions until fresh Luna signature reviews score both clarity and cohesion 3/3.
---

# Signature refactor

- Follow [the review workflow](../../../docs/signature-review.md). Run
  `go run ./scripts/signatures` from the repository root; keep inventories and
  reports outside the working tree.
- Delegate each round to fresh independent Luna agents (`gpt-6-luna`,
  `fork_turns="none"`). Give them the rubric and the complete frozen inventory,
  including file headers, but no source, documentation, diffs, or prior reviews.
  They score responsibility/inputs/outputs together and cohesion separately
  from 1 to 3; cohesion below 3 identifies every affected level with `S/F/P`.
- Verify copied inventory bytes and SHA-256 digests, exactly one judgment per
  signature (no file headers), valid scores, and reasons for every score below
  3. Reject invalid rounds. Do not automate judgments or steer scores.
- Refactor every finding while preserving behavior; check that names match
  implementations and fix responsibility boundaries rather than just renaming.
  Exclude tests, test-only helpers, and fixtures; only update their production
  references as needed. Repeat until a complete fresh round gives every
  signature 3/3. If Luna is unavailable, report the blocker without claiming
  completion.
- Confirm extraction still matches the passing round. Run `make check`,
  `make test`, and `make build`; remove task-generated reports and artifacts.
  Summarize changes and validation. Commit and push the current branch only
  when requested; do not restore unrelated changes unless instructed.
