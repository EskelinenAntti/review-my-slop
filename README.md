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
select a range, `c` to comment, and `s` to submit the review.

The AI-facing `review-my-comments` command prints and consumes all pending
feedback for its current repository. Re-run it to retrieve comments submitted
while the AI was working.

Run `?` in the TUI for the complete key map.

