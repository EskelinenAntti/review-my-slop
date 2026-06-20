# review-my-slop

**Review the code, not the chat transcript.**

Coding has moved from the editor to the chat window. Agents can produce code at
a pace that makes reading the code feel like an unnecessary bottleneck. It is
not. Without supervision, agents tend to slowly accumulate technical debt. That
debt increases the likelihood of mistakes, which in turn create more debt and
hidden bugs. By the time an agent burns through tokens on simple changes, it is
usually too late to correct course.

This is not simply an agent problem. Successful collaboration between the
developer and the agent requires a shared understanding of the codebase.

`review-my-slop` is a keyboard-driven terminal diff viewer for reviewing local
changes, attaching comments to exact lines or ranges, and handing that feedback
back to the agent with one command.

## The workflow

1. Prompt the agent to make the initial change.
2. Review that slop by running `review-my-slop code` in the repository.
3. Read the diff and attach comments to individual lines or ranges.
4. Let the agent review your slop comments by asking it to run
   `review-my-slop comments` and act on them.
5. Repeat from step 2 until the code is no longer slop.

The review UI uses Vim-like key bindings for navigation, selection, search, and
comments. Press `?` at any time to see the complete key map.

All data stays truly local, and no telemetry is sent.

## Install

```sh
brew install EskelinenAntti/cli/review-my-slop
```

The project is licensed under the [MIT License](LICENSE).
