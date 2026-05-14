# Review My Slop

RMS (Review My Slop) is the review tool you need to review your and your colleagues slop.

It is a simple wrapper command around difftastic which in addition allows you to
1. Select lines from diff and open them in your `$EDITOR`
2. Leave comments locally (those will be saved in json file with line numbers)

Its written in Go.

## Usage

```sh
go run .
```

RMS shows your current `git diff` through difftastic when `difft` is
installed, then opens an interactive review view.

Keys:

- `j` / `k` move the selected changed line
- `c` leaves a local comment for the selected line
- `e` or `Enter` opens the selected line in `$VISUAL` or `$EDITOR`
- `g` / `G` jumps to the first or last changed line
- `Ctrl-u` / `Ctrl-d` moves by half a page
- `r` reloads the diff
- `q` quits

Comments are saved to `.rms-comments.json`.
