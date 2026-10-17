package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func TestRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
	pending := comments.Comment{Repository: p.Root, Body: "pending"}
	if _, err := store.Add(pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := func() error {
		var refresh func(context.Context, patch.Kind) (patch.Patch, error) = func(
			received context.Context,
			kind patch.Kind,
		) (patch.Patch, error) {
			testutil.RequireEqual(t, received, ctx, "context=%v kind=%v", received,
				kind,
			)
			testutil.RequireEqual(t, kind, p.Kind, "context=%v kind=%v", received,
				kind,
			)

			return p, nil
		}
		pending, err := commentStore(store).List(p.Root)
		if err != nil {
			return err
		}
		dimensions := func() size { return size{Width: 120, Height: 40} }()
		m, err := func() (model, error) {
			preferences, err := settings.Load()
			if err != nil {
				return model{}, err
			}
			m := func() model {
				var layout initialLayout = initialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) }, size: dimensions}
				inlineLocalSize1 := layout.size
				if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
					inlineLocalSize1 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: pending}, commentView: commentscreen.New(pending), edit: commentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
					comment.Repository = current.Root
					if comment.ID != "" {
						if err := commentStore(store).Update(comment); err != nil {
							return comments.Comment{}, err
						}
						return comment, nil
					}
					return commentStore(store).Add(comment)
				}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(p, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			m.delete = func(comment comments.Comment, current patch.Patch) error {
				return commentStore(store).Delete(current.Root, comment.ID)
			}
			m.load = func() ([]comments.Comment, error) { return commentStore(store).List(p.Root) }
			return m, nil
		}()
		if err != nil {
			return err
		}
		m.ctx = ctx
		if refresh != nil {
			m.refresh = func(kind patch.Kind) (patch.Patch, error) { return refresh(ctx, kind) }
		}
		return func(m model, _ ...tea.ProgramOption) error {
			testutil.RequireEqual(t, m.ctx, ctx, "model=%#v", m)
			testutil.RequireEqual(t, m.width, 120, "model=%#v", m)
			testutil.RequireEqual(t, m.height, 40, "model=%#v", m)
			testutil.RequireEqual(t, len(m.comments.items), 1, "model=%#v", m)
			result := func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
			}()()
			msg, ok := result.(refreshDiffMsg)
			testutil.RequireEqual(t, ok, true, "refresh message=%T", result)
			testutil.RequireEqual(t, msg.err, nil, "%s", msg.err)
			return sentinel
		}(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
	}()
	testutil.RequireEqual(t, errors.Is(err, sentinel), true, "error=%v", err)
}

func TestRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("HOME", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			config, err := os.UserConfigDir()
			testutil.NoError(t, err)
			testutil.NoError(t, os.MkdirAll(config, 0700))

			store := comments.Store{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
			{
				var config string = config
				t.Helper()
				if failure == "comments" {
					testutil.NoError(t, os.WriteFile(store.Path, []byte("invalid database"), 0600))
				} else {
					testutil.NoError(t, os.Mkdir(filepath.Join(config, "review-my-slop"), 0700))
					testutil.NoError(t, os.WriteFile(filepath.Join(config, "review-my-slop", "ui.json"), []byte("invalid json"), 0600))
				}
			}
			err = func() error {
				var (
					ctx          context.Context = context.Background()
					currentPatch patch.Patch     = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
				)
				pending, err := commentStore(store).List(currentPatch.Root)
				if err != nil {
					return err
				}
				dimensions := func() size { return defaultSize }()
				m, err := func() (model, error) {
					preferences, err := settings.Load()
					if err != nil {
						return model{}, err
					}
					m := func() model {
						var layout initialLayout = initialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) }, size: dimensions}
						inlineLocalSize2 := layout.size
						if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
							inlineLocalSize2 = defaultSize
						}
						m := model{ctx: context.Background(), currentPatch: currentPatch, comments: commentState{items: pending}, commentView: commentscreen.New(pending), edit: commentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
							comment.Repository = current.Root
							if comment.ID != "" {
								if err := commentStore(store).Update(comment); err != nil {
									return comments.Comment{}, err
								}
								return comment, nil
							}
							return commentStore(store).Add(comment)
						}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
						m.diffView = diffscreen.New(currentPatch, m.diffOptions)
						m.diffView.Resize(m.width, m.height)
						m.commentView.Resize(m.width, m.height)
						return m
					}()
					m.delete = func(comment comments.Comment, current patch.Patch) error {
						return commentStore(store).Delete(current.Root, comment.ID)
					}
					m.load = func() ([]comments.Comment, error) { return commentStore(store).List(currentPatch.Root) }
					return m, nil
				}()
				if err != nil {
					return err
				}
				m.ctx = ctx
				if (func(context.Context, patch.Kind) (patch.Patch, error))(nil) != nil {
					m.refresh = func(kind patch.Kind) (patch.Patch, error) {
						return (func(context.Context, patch.Kind) (patch.Patch, error))(nil)(ctx, kind)
					}
				}
				return func(model, ...tea.ProgramOption) error { t.Fatal("program started after startup failure"); return nil }(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
			}()
			testutil.RequireEqual(t, err != nil, true, "%s",
				"expected startup error",
			)
		})
	}
}

func TestRunContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := func() error {
		var (
			currentPatch patch.Patch  = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			store        commentStore = comments.Store{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
		)
		pending, err := store.List(currentPatch.Root)
		if err != nil {
			return err
		}
		dimensions := func() size { return defaultSize }()
		m, err := func() (model, error) {
			preferences, err := settings.Load()
			if err != nil {
				return model{}, err
			}
			m := func() model {
				var layout initialLayout = initialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) }, size: dimensions}
				inlineLocalSize3 := layout.size
				if inlineLocalSize3.Width <= 0 || inlineLocalSize3.Height <= 0 {
					inlineLocalSize3 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: currentPatch, comments: commentState{items: pending}, commentView: commentscreen.New(pending), edit: commentEdit{index: -1}, width: inlineLocalSize3.Width, height: inlineLocalSize3.Height, save: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
					comment.Repository = current.Root
					if comment.ID != "" {
						if err := store.Update(comment); err != nil {
							return comments.Comment{}, err
						}
						return comment, nil
					}
					return store.Add(comment)
				}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(currentPatch, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			m.delete = func(comment comments.Comment, current patch.Patch) error {
				return store.Delete(current.Root, comment.ID)
			}
			m.load = func() ([]comments.Comment, error) { return store.List(currentPatch.Root) }
			return m, nil
		}()
		if err != nil {
			return err
		}
		m.ctx = ctx
		if (func(context.Context, patch.Kind) (patch.Patch, error))(nil) != nil {
			m.refresh = func(kind patch.Kind) (patch.Patch, error) {
				return (func(context.Context, patch.Kind) (patch.Patch, error))(nil)(ctx, kind)
			}
		}
		return func(m model, options ...tea.ProgramOption) error {
			options = append(options, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
			_, err := tea.NewProgram(m, options...).Run()
			return err
		}(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
	}()
	testutil.RequireEqual(t, errors.Is(err, tea.ErrProgramKilled), true,
		"error=%v", err,
	)
}

func TestTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, tc := range []struct {
		name           string
		success, calls int
		want           size
	}{
		{"stdin", 0, 1, size{Width: 120, Height: 40}},
		{"stdout", 1, 2, size{Width: 120, Height: 40}},
		{"fallback", 2, 2, defaultSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := terminalSizeProbe{t: t, success: tc.success}
			got := func() size {
				if width, height, err := func() (width, height int, err error) {
					var (
						p  *terminalSizeProbe = &probe
						fd uintptr            = os.Stdin.Fd()
					)
					p.t.Helper()
					descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
					testutil.RequireEqual(p.t, p.calls < len(descriptors), true, "unexpected terminal size call: %d", p.calls)
					testutil.RequireEqual(p.t, fd, descriptors[p.calls], "fd=%v want=%v", fd, descriptors[p.calls])
					current := p.calls
					p.calls++
					if current == p.success {
						return 120, 40, nil
					}
					return 0, 0, errors.New("not a terminal")
				}(); err == nil {
					return size{Width: width, Height: height}
				}
				if width, height, err := func() (width, height int, err error) {
					var (
						p  *terminalSizeProbe = &probe
						fd uintptr            = os.Stdout.Fd()
					)
					p.t.Helper()
					descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
					testutil.RequireEqual(p.t, p.calls < len(descriptors), true, "unexpected terminal size call: %d", p.calls)
					testutil.RequireEqual(p.t, fd, descriptors[p.calls], "fd=%v want=%v", fd, descriptors[p.calls])
					current := p.calls
					p.calls++
					if current == p.success {
						return 120, 40, nil
					}
					return 0, 0, errors.New("not a terminal")
				}(); err == nil {
					return size{Width: width, Height: height}
				}
				return defaultSize
			}()
			testutil.RequireEqual(t, got, tc.want, "size=%v want=%v", got,
				tc.want,
			)
			testutil.RequireEqual(t, probe.calls, tc.calls, "calls=%d want=%d",
				probe.calls, tc.calls,
			)
		})
	}
}

type terminalSizeProbe struct {
	t              *testing.T
	success, calls int
}
