package ui

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
)

func TestRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := modelPatch()
	if _, err := store.Add(comments.Comment{Repository: p.Root, Body: "pending"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := run(ctx, p, store, func(received context.Context, kind patch.Kind) (patch.Patch, error) {
		if received != ctx || kind != p.Kind {
			t.Fatalf("context=%v kind=%v", received, kind)
		}
		return p, nil
	}, func() size { return size{Width: 120, Height: 40} }, func(m model, options ...tea.ProgramOption) error {
		if m.ctx != ctx || m.width != 120 || m.height != 40 || len(m.comments.items) != 1 {
			t.Fatalf("model=%#v", m)
		}
		if msg := m.loadRefresh()().(refreshDiffMsg); msg.err != nil {
			t.Fatal(msg.err)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v", err)
	}
}

func TestRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("HOME", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			config, err := os.UserConfigDir()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(config, 0700); err != nil {
				t.Fatal(err)
			}
			store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			if failure == "comments" {
				if err := os.WriteFile(store.Path, []byte("invalid database"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(config, "review-my-slop"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(config, "review-my-slop", "ui.json"), []byte("invalid json"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err = run(context.Background(), modelPatch(), store, nil, func() size { return defaultSize }, func(model, ...tea.ProgramOption) error { t.Fatal("program started after startup failure"); return nil })
			if err == nil {
				t.Fatal("expected startup error")
			}
		})
	}
}

func TestRunContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(ctx, modelPatch(), comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}, nil, func() size { return defaultSize }, func(m model, options ...tea.ProgramOption) error {
		options = append(options, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
		_, err := tea.NewProgram(m, options...).Run()
		return err
	})
	if !errors.Is(err, tea.ErrProgramKilled) {
		t.Fatalf("error=%v", err)
	}
}

func TestTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, success := range []int{0, 1, 2} {
		calls := 0
		got := discoverTerminalSize(func(fd uintptr) (int, int, error) {
			expected := os.Stdin.Fd()
			if calls == 1 {
				expected = os.Stdout.Fd()
			}
			if fd != expected {
				t.Fatalf("fd=%v want=%v", fd, expected)
			}
			current := calls
			calls++
			if current == success {
				return 120, 40, nil
			}
			return 0, 0, errors.New("not a terminal")
		})
		want := size{Width: 120, Height: 40}
		if success == 2 {
			want = defaultSize
		}
		if got != want || calls != min(success+1, 2) {
			t.Fatalf("size=%v calls=%d", got, calls)
		}
	}
}
