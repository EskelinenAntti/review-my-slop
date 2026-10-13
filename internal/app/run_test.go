package app

import (
	"context"
	"errors"
	"fmt"
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
	runtime := programRuntime{
		terminalSize: func() size {
			return size{
				Width:  120,
				Height: 40,
			}
		},
		launch: func(m model, _ ...tea.ProgramOption) error {
			testutil.RequireEqual(t, m.ctx, ctx, fmt.Sprintf("model=%#v", m))
			testutil.RequireEqual(t, m.width, 120, fmt.Sprintf("model=%#v", m))
			testutil.RequireEqual(t, m.height, 40, fmt.Sprintf("model=%#v", m))
			testutil.RequireEqual(t, len(m.comments.items), 1,
				fmt.Sprintf("model=%#v", m),
			)

			result := m.loadRefresh()()
			msg, ok := result.(refreshDiffMsg)
			testutil.RequireEqual(t, ok, true,
				fmt.Sprintf("refresh message=%T", result),
			)
			testutil.RequireEqual(t, msg.err, nil, fmt.Sprintf("%s", msg.err))

			return sentinel
		},
	}
	err := runtime.start(ctx, p, store, func(
		received context.Context,
		kind patch.Kind,
	) (patch.Patch, error) {
		testutil.RequireEqual(t, received, ctx,
			fmt.Sprintf("context=%v kind=%v", received, kind),
		)
		testutil.RequireEqual(t, kind, p.Kind,
			fmt.Sprintf("context=%v kind=%v", received, kind),
		)

		return p, nil
	})
	testutil.RequireEqual(t, errors.Is(err, sentinel), true,
		fmt.Sprintf("error=%v", err),
	)
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
			runtime := programRuntime{
				terminalSize: func() size { return defaultSize },
				launch: func(
					model,
					...tea.ProgramOption,
				) error {
					t.Fatal("program started after startup failure")
					return nil
				},
			}
			err = runtime.start(context.Background(), modelPatch(), store, nil)
			testutil.RequireEqual(t, err != nil, true,
				"expected startup error",
			)
		})
	}
}

func TestRunContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := programRuntime{
		terminalSize: func() size { return defaultSize },
		launch: func(
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
		},
	}
	err := runtime.start(ctx, modelPatch(), comments.Store{
		Path: filepath.Join(t.TempDir(), "comments.db"),
	}, nil)
	testutil.RequireEqual(t, errors.Is(err, tea.ErrProgramKilled), true,
		fmt.Sprintf("error=%v", err),
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
			got := discoverTerminalSize(probe.getSize)
			testutil.RequireEqual(t, got, tc.want,
				fmt.Sprintf("size=%v want=%v", got, tc.want),
			)
			testutil.RequireEqual(t, probe.calls, tc.calls,
				fmt.Sprintf("calls=%d want=%d", probe.calls, tc.calls),
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
		fmt.Sprintf("unexpected terminal size call: %d", p.calls),
	)
	testutil.RequireEqual(p.t, fd, descriptors[p.calls],
		fmt.Sprintf("fd=%v want=%v", fd, descriptors[p.calls]),
	)
	current := p.calls
	p.calls++
	if current == p.success {
		return 120, 40, nil
	}
	return 0, 0, errors.New("not a terminal")
}
