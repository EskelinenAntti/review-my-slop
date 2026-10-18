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
	pending := comments.Comment{Repository: p.Root, Body: "pending"}
	if _, err := store.Add(pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := start(ctx, p, store, func(
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
	}, func() terminalDimensions {
		return terminalDimensions{
			width:  120,
			height: 40,
		}
	}, func(m model, _ ...tea.ProgramOption) error {
		testutil.RequireEqual(t, m.ctx, ctx, "model=%#v", m)
		testutil.RequireEqual(t, m.dimensions.width, 120, "model=%#v", m)
		testutil.RequireEqual(t, m.dimensions.height, 40, "model=%#v", m)
		testutil.RequireEqual(t, len(m.comments.items), 1, "model=%#v", m)

		result := m.loadRefresh()()
		msg, ok := result.(refreshDiffMsg)
		testutil.RequireEqual(t, ok, true, "refresh message=%T", result)
		testutil.RequireEqual(t, msg.err, nil, "%s", msg.err)

		return sentinel
	})
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
			prepareStartupFailure(t, failure, config, store.Path)
			err = start(
				context.Background(),
				modelPatch(),
				store,
				nil,
				func() terminalDimensions { return defaultDimensions },
				func(
					model,
					...tea.ProgramOption,
				) error {
					t.Fatal("program started after startup failure")
					return nil
				},
			)
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
	err := start(ctx, modelPatch(), comments.Store{
		Path: filepath.Join(t.TempDir(), "comments.db"),
	}, nil, func() terminalDimensions { return defaultDimensions }, func(
		m model,
		options ...tea.ProgramOption,
	) error {
		options = append(
			options,
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithoutRenderer(),
		)
		_, err := tea.NewProgram(m, options...).Run()
		return err
	})
	testutil.RequireEqual(t, errors.Is(err, tea.ErrProgramKilled), true,
		"error=%v", err,
	)
}

func TestTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, tc := range []struct {
		name           string
		success, calls int
		want           terminalDimensions
	}{
		{"stdin", 0, 1, terminalDimensions{width: 120, height: 40}},
		{"stdout", 1, 2, terminalDimensions{width: 120, height: 40}},
		{"fallback", 2, 2, defaultDimensions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := terminalSizeProbe{t: t, success: tc.success}
			got := discoverTerminalSize(probe.getSize)
			testutil.RequireEqual(t, got, tc.want, "size=%v want=%v", got,
				tc.want,
			)
			testutil.RequireEqual(t, probe.calls, tc.calls, "calls=%d want=%d",
				probe.calls, tc.calls,
			)
		})
	}
}

func prepareStartupFailure(t *testing.T, failure, config, database string) {
	t.Helper()
	if failure == "comments" {
		testutil.NoError(t,
			os.WriteFile(database, []byte("invalid database"), 0600),
		)
	} else {
		testutil.NoError(t,
			os.Mkdir(filepath.Join(config, "review-my-slop"), 0700),
		)
		testutil.NoError(
			t,
			os.WriteFile(
				filepath.Join(config, "review-my-slop", "ui.json"),
				[]byte("invalid json"),
				0600,
			),
		)
	}
}

type terminalSizeProbe struct {
	t              *testing.T
	success, calls int
}

func (p *terminalSizeProbe) getSize(fd uintptr) (width, height int, err error) {
	p.t.Helper()
	descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
	testutil.RequireEqual(p.t, p.calls < len(descriptors), true,
		"unexpected terminal size call: %d", p.calls,
	)
	testutil.RequireEqual(p.t, fd, descriptors[p.calls], "fd=%v want=%v", fd,
		descriptors[p.calls],
	)
	current := p.calls
	p.calls++
	if current == p.success {
		return 120, 40, nil
	}
	return 0, 0, errors.New("not a terminal")
}
