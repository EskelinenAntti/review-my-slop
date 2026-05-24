# Review My Slop

Review My Slop is the review tool you need to review your and your colleagues slop.

It is a simple wrapper command around difftastic which in addition allows you to
select lines from diff and open them in your `$EDITOR`.

It's written in Go.

## Usage

```sh
slop
```

Review My Slop requires `difft` and shows the current `git diff` through
difftastic, then opens an interactive review view. Pass the same arguments you
would pass to `git diff`.

If the current branch has an active GitHub pull request and `gh` is
authenticated, Review My Slop can create a pending GitHub PR review, add draft
comments and suggestions to it, open the PR, and submit the draft as one review.

## Git configuration

Option 1: add `git slop` as an alias.

```sh
git config --global alias.slop '!slop'
```

Then run:

```sh
git slop
git slop --staged
git slop main...HEAD
```

Option 2: replace `git diff` through Git's external diff configuration.

For one-off usage:

```sh
git -c diff.external=slop diff
```

To make `git diff` use Review My Slop by default, add this to `~/.gitconfig`:

```gitconfig
[diff]
    external = slop
```

Then run `git diff`, `git diff --staged`, or `git diff main...HEAD` as usual.
Other Git commands that show diffs still need `--ext-diff`, for example
`git show --ext-diff` or `git log -p --ext-diff`.

To opt out for one command, use `--no-ext-diff`:

```sh
git diff --no-ext-diff
```

Keys:

- `j` / `k` move the selected changed line
- `h` / `l` choose the left or right side of a side-by-side changed row
- `v` starts or clears a visual selection for a same-file, same-side range while
  reviewing branch changes
- `R` starts a pending GitHub PR review while reviewing branch changes
- `c` opens `$VISUAL` or `$EDITOR` and adds the body as a draft comment when
  review mode is active
- `s` opens `$VISUAL` or `$EDITOR` with a prefilled `suggestion` block for the
  right side, then adds it as a draft comment when review mode is active
- `P` submits the pending review, opening `$VISUAL` or `$EDITOR` for an optional
  review summary
- `D` deletes the pending GitHub review
- `e` or `Enter` opens the selected line in `$VISUAL` or `$EDITOR`
- `o` opens the current GitHub PR in the browser
- `g` / `G` jumps to the first or last changed line
- `Ctrl-u` / `Ctrl-d` moves by half a page
- `r` reloads the diff
- `Esc` clears an active visual selection or quits when no selection is active
- `q` quits
