# Refactoring guide

Apply these improvements when evidence in this repository supports them.
Numbers are stable references for commit bodies, not a mandatory execution order.
This is a practical, paraphrased checklist of the books' design advice, rather
than a reproduction of their text or examples. Case studies illustrate these
principles rather than adding mandatory architectures for this Go application.

Priority: **A Philosophy of Software Design**, then **Tidy First?**, then
**Clean Code**. Favor reduced cognitive load over short functions. Extract only
operations with clear responsibilities and intuitive English names. Preserve
behavior in refactoring commits; report intentional behavior changes separately.
Never change Revive rules or other linter configuration.

## A Philosophy of Software Design — John Ousterhout

Reference: [author's book page and sample chapters](https://web.stanford.edu/~ouster/cgi-bin/book.php).

1. Reduce change amplification.
2. Reduce cognitive load.
3. Expose hidden dependencies.
4. Invest in strategic design continuously.
5. Prefer deep modules.
6. Simplify interfaces.
7. Hide implementation decisions.
8. Localize shared knowledge.
9. Avoid temporal decomposition.
10. Make common operations simple.
11. Prefer useful generality over special cases.
12. Separate general mechanisms from special policies.
13. Keep adjacent abstraction levels distinct.
14. Remove unnecessary forwarding.
15. Remove pass-through parameters.
16. Keep related responsibilities together.
17. Separate unrelated responsibilities.
18. Pull complexity into implementations.
19. Define avoidable errors out of existence.
20. Handle related failures together.
21. Recover where recovery is meaningful.
22. Compare alternative designs.
23. Document contracts and invariants.
24. Explain intent and nonobvious implementation decisions.
25. Avoid comments repeating code.
26. Choose precise, consistent names.
27. Write interface comments before implementation.
28. Update comments alongside changes.
29. Make behavior obvious.
30. Maintain consistent conventions.
31. Measure before optimizing.
32. Simplify critical paths.
33. Recognize complexity warning signs early.

## Tidy First? — Kent Beck

Reference: [publisher's contents](https://www.oreilly.com/library/view/tidy-first/9781098151232/).

34. Use guard clauses.
35. Delete dead code.
36. Normalize equivalent forms.
37. Introduce improved interfaces over existing implementations.
38. Arrange code for reading.
39. Group related code.
40. Initialize at declaration.
41. Introduce explaining variables.
42. Introduce explaining constants.
43. Pass dependencies explicitly.
44. Separate statement groups visually.
45. Extract cohesive helpers.
46. Recombine fragmented logic.
47. Add explanatory comments.
48. Delete redundant comments.
49. Separate structural and behavioral changes.
50. Chain small tidyings.
51. Keep batches reviewable.
52. Alternate tidying and behavior work deliberately.
53. Untangle changes incrementally.
54. Choose first, after, later, or never deliberately.
55. Improve relationships between elements.
56. Distinguish structure from behavior.
57. Balance immediate value against future options.
58. Prefer reversible structural changes.
59. Reduce costly coupling.
60. Weigh decoupling against its cost.
61. Increase cohesion.

## Clean Code — Robert C. Martin, original edition

Reference: [publisher's contents](https://www.oreilly.com/library/view/clean-code-a/9780136083238/).

62. Leave touched code clearer.
63. Use intention-revealing names.
64. Avoid misleading names and encodings.
65. Make meaningful distinctions.
66. Use searchable, pronounceable names.
67. Use consistent domain vocabulary.
68. Supply necessary naming context.
69. Keep functions focused.
70. Keep abstraction levels consistent.
71. Minimize arguments.
72. Replace confusing flag arguments.
73. Avoid hidden side effects.
74. Separate commands from queries.
75. Remove duplication.
76. Prefer clear control flow.
77. Explain intent, consequences, and contracts.
78. Delete stale or misleading comments.
79. Remove commented-out code.
80. Maintain readable formatting and locality.
81. Hide representation behind behavior.
82. Distinguish objects from data records.
83. Avoid navigation through internal structures.
84. Provide contextual errors.
85. Separate error handling from normal work.
86. Avoid error-code plumbing and ambiguous nulls.
87. Isolate external dependencies.
88. Characterize boundary behavior with tests.
89. Keep tests readable and independent.
90. Test boundaries and failure cases.
91. Keep tests fast, repeatable, and self-checking.
92. Give modules cohesive responsibilities.
93. Encapsulate mutable state.
94. Depend on stable interfaces where useful.
95. Separate construction from use.
96. Avoid speculative infrastructure.
97. Express intent with minimal duplication.
98. Keep concurrency responsibilities separate.
99. Minimize shared mutable data.
100. Bound synchronization and protect shutdown.
101. Exercise concurrent failures and interleavings.
102. Remove obsolete dependencies and unused flexibility.
103. Replace unexplained literals with meaningful names.
104. Make ordering dependencies explicit.
105. Place decisions at the appropriate abstraction level.
106. Make boundary conditions explicit.

## Commit and verification workflow

- Inspect callers, tests, and persistence or UI contracts before choosing a change.
- Apply one cohesive refactoring per commit; cite the relevant numbers and explain
  the concrete change in the commit message body.
- Use existing behavioral coverage; add tests when a meaningful contract lacks it.
- Run `make test` and `make check` before committing code changes.
- Push immediately after each commit. Open a PR after the first commit and keep
  its description aligned with the accumulated changes.
- Treat the list as review criteria, not a demand to introduce every technique.
  In particular, deep modules outrank excessive extraction and speculative
  interfaces; Go error values remain idiomatic.
