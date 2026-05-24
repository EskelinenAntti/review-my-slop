# Tech Debt

Add architectural issues here when they materially slow an iteration down, require excessive searching or editing, or make changes prone to breakage. Leave this file otherwise empty apart from this instruction.

- Diff source and review capability are still coupled through `sourceLocal` and `sourceBranch`. Removing the `Tab` source switch exposed that branch-review behavior and multi-line selection are keyed to an internal source mode instead of the actual diff arguments, which makes a pure `git diff`-style interface harder to reason about.
