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
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := modelPatch()
	{
		_, err := store.Add(comments.Comment{Repository: p.Root, Body: "pending"})
		testutil.Require(t, err == nil, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := run(ctx, p, store, func(received context.Context, kind patch.Kind) (patch.Patch, error) {
		testutil.Requiref(t, received == ctx && kind == p.Kind, "context=%v kind=%v", received, kind)
		return p, nil
	}, func() size { return size{Width: 120, Height: 40} }, func(m model, options ...tea.ProgramOption) error {
		testutil.Requiref(t, m.ctx == ctx && m.width == 120 && m.height == 40 && len(m.comments.items) == 1, "model=%#v", m)
		{
			msg := m.loadRefresh()().(refreshDiffMsg)
			testutil.Require(t, msg.err == nil, msg.err)
		}
		return sentinel
	})
	testutil.Requiref(t, errors.Is(err, sentinel), "error=%v", err)
}

func TestRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("HOME", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			config, err := os.UserConfigDir()
			testutil.Require(t, err == nil, err)

			testutil.NoError(t, os.MkdirAll(config, 0700))

			store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			writeStartupFailure(t, failure, config, store.Path)
			err = run(context.Background(), modelPatch(), store, nil, func() size { return defaultSize }, func(model, ...tea.ProgramOption) error { t.Fatal("program started after startup failure"); return nil })
			testutil.Require(t, err != nil, "expected startup error")
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
	testutil.Requiref(t, errors.Is(err, tea.ErrProgramKilled), "error=%v", err)
}

func TestTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, success := range []int{0, 1, 2} {
		checkTerminalSizeDiscovery(t, success)
	}
}

func checkTerminalSizeDiscovery(t *testing.T, success int) {
	t.Helper()
	calls := 0
	got := discoverTerminalSize(terminalSizeProbe(t, success, &calls))
	want := size{Width: 120, Height: 40}
	if success == 2 {
		want = defaultSize
	}
	testutil.Requiref(t, got == want && calls == min(success+1, 2), "size=%v calls=%d", got, calls)
}

func terminalSizeProbe(t *testing.T, success int, calls *int) func(uintptr) (int, int, error) {
	return func(fd uintptr) (int, int, error) { return probeTerminalSize(t, fd, success, calls) }
}

func probeTerminalSize(t *testing.T, fd uintptr, success int, calls *int) (int, int, error) {
	t.Helper()
	expected := os.Stdin.Fd()
	if *calls == 1 {
		expected = os.Stdout.Fd()
	}
	testutil.Requiref(t, fd == expected, "fd=%v want=%v", fd, expected)
	current := *calls
	*calls++
	if current == success {
		return 120, 40, nil
	}
	return 0, 0, errors.New("not a terminal")
}

func writeStartupFailure(t *testing.T, failure, config, databasePath string) {
	t.Helper()
	if failure == "comments" {

		testutil.NoError(t, os.WriteFile(databasePath, []byte("invalid database"), 0600))

	} else {

		testutil.NoError(t, os.Mkdir(filepath.Join(config, "review-my-slop"), 0700))

		testutil.NoError(t, os.WriteFile(filepath.Join(config, "review-my-slop", "ui.json"), []byte("invalid json"), 0600))

	}
}
