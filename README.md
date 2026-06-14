# review-my-slop

`review-my-slop` is a terminal UI for reviewing unstaged and untracked Git
changes and attaching feedback to changed lines. Feedback is stored outside the
repository and can be pulled into an AI coding session with:

```text
!review-my-slop comments
```

Supported platforms are macOS and Linux on amd64 and arm64. The program
requires `git`, a POSIX shell, and a terminal.

## Install from source

```sh
go install github.com/eskelinenantti/review-my-slop/cmd/review-my-slop@latest
```

## Usage

Run `review-my-slop` or `review-my-slop code` in a Git repository. Use `j` and
`k` to move, `v` to select a range, and `c` to comment in `$EDITOR` using a
temporary Markdown file. The file includes the selected lines in a fenced
`suggestion` block for reference; the unchanged block is removed before the
comment is saved.
Saving an empty file discards the comment, including when editing an existing
comment. Press `C` to browse pending comments, edit one with `Enter`, or delete
one with `D`.
Long lines can be scrolled four columns at a time with `h`/`l` or the
left/right arrow keys;
`0` and `$` jump to the horizontal start and end. Press `e` to open the current
line in `$EDITOR`. The diff refreshes when the terminal gains focus and keeps
the cursor on the same changed line when possible; press `R` to refresh it
manually. Press `Tab` to cycle through local changes and available parent
branches, ordered from the nearest stacked parent to the default branch. Each
branch view compares that branch's merge-base commit with the worktree, so
committed, staged, unstaged, and untracked changes are all included. Press `/`
to search the diff, then use `n` and `N` to move between matches.
Press `o` to open the current branch's pull request in a browser using `gh`.

Run `review-my-slop pr 123` to check out pull request 123 with the GitHub CLI
and review it against its base branch. Comments created in this mode are saved
to your pending GitHub pull request review instead of the local inbox. The
command requires an authenticated `gh` installation.

The AI-facing `review-my-slop comments` command prints and consumes all pending
feedback for its current repository. Re-run it to retrieve comments saved while
the AI was working.

## Example AI skill

[`examples/skills/comments`](examples/skills/comments) contains an example
`comments` skill that teaches an AI coding agent to retrieve, address, verify,
and re-check review feedback. Copy that directory into your agent's skills
directory, then invoke it as `/comments` (or `$comments` in agents that use
dollar-prefixed skill names).

Pending comments are stored in
`$XDG_DATA_HOME/review-my-slop/inbox.db`, defaulting to
`~/.local/share/review-my-slop/inbox.db`. Temporary editor drafts use
`$XDG_STATE_HOME/review-my-slop`, defaulting to
`~/.local/state/review-my-slop`.

Run `?` in the TUI for the complete key map.
