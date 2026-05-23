# Review My Slop

RMS (Review My Slop) is the review tool you need to review your and your colleagues slop.

It is a simple wrapper command around difftastic which in addition allows you to
select lines from diff and open them in your `$EDITOR`.

Its written in Go.

## Usage

```sh
go run .
```

RMS requires `difft` and shows your current `git diff` through difftastic, then
opens an interactive review view. If the current branch has an active GitHub pull
request and `gh` is authenticated, RMS can create a pending GitHub PR review,
add draft comments and suggestions to it, and submit them as one review.

RMS starts on local uncommitted changes. When PR context is available, `Tab`
switches between local changes and branch changes.

Keys:

- `j` / `k` move the selected changed line
- `h` / `l` choose the left or right side of a side-by-side changed row
- `Tab` switches between local changes and branch changes when both are available
- `v` starts or clears a visual selection for a same-file, same-side range while
  reviewing branch changes
- `R` starts a pending GitHub PR review
- `c` opens `$VISUAL` or `$EDITOR` and adds the body as a draft comment when
  review mode is active
- `s` opens `$VISUAL` or `$EDITOR` with a prefilled `suggestion` block for the
  right side, then adds it as a draft comment when review mode is active
- `p` submits the pending review, opening `$VISUAL` or `$EDITOR` for an optional
  review summary
- `D` deletes the pending GitHub review
- `e` or `Enter` opens the selected line in `$VISUAL` or `$EDITOR`
- `g` / `G` jumps to the first or last changed line
- `Ctrl-u` / `Ctrl-d` moves by half a page
- `r` reloads the diff
- `Esc` clears an active visual selection or quits when no selection is active
- `q` quits

## Codex Flow

```sh
go run . flow
```

The `flow` command runs the product loop:

1. Opens `$VISUAL` or `$EDITOR` for the initial prompt and uses it as the draft
   PR description.
2. Uses the description title as the PR title, creates an `rms/...`
   kebab-cased branch from that title, runs `codex exec` with the prompt on
   stdin, commits the result, pushes, and opens a draft PR.
3. Opens the RMS review UI so you can leave and submit GitHub review comments.
4. Reads unresolved review threads, sends them back through `codex exec`, commits
   and pushes the fixes, then resolves the addressed threads.
5. Repeats until there are no unresolved comments and you leave the follow-up
   prompt empty, write `READY`, or write `no more changes`; then it marks the PR
   ready for review.

This uses the installed Codex CLI, not the Codex API directly. `codex exec -`
reads instructions from stdin, which is the subprocess interface used here.
