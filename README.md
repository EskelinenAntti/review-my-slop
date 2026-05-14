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
opens an interactive review view.

Keys:

- `j` / `k` move the selected changed line
- `e` or `Enter` opens the selected line in `$VISUAL` or `$EDITOR`
- `g` / `G` jumps to the first or last changed line
- `Ctrl-u` / `Ctrl-d` moves by half a page
- `r` reloads the diff
- `q` quits
