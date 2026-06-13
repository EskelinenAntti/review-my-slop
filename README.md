# review-my-slop

`review-my-slop` is a terminal UI for reviewing unstaged and untracked Git
changes and attaching feedback to changed lines. Feedback is stored outside the
repository and can be pulled into an AI coding session with:

```text
!review-my-comments
```

## Install from source

```sh
go install github.com/anttieskelinen/review-my-slop/cmd/review-my-slop@latest
go install github.com/anttieskelinen/review-my-slop/cmd/review-my-comments@latest
```

## Usage

Run `review-my-slop` in a Git repository. Use `j` and `k` to move, `v` to
select a range, and `c` to comment. Press `Enter` to save each comment directly
for the AI, `Shift+Enter` for a line break, or `Ctrl+G` to edit it in
`$EDITOR`. Press `C` to browse pending comments and edit one with `Enter`.
Long lines can be scrolled with `h`/`l` or the left/right arrow keys;
`0` and `$` jump to the horizontal start and end. Press `e` to open the current
line in `$EDITOR`. The diff refreshes as files change and keeps the cursor on
the same changed line when possible.

The AI-facing `review-my-comments` command prints and consumes all pending
feedback for its current repository. Re-run it to retrieve comments saved
while the AI was working.

Run `?` in the TUI for the complete key map.
