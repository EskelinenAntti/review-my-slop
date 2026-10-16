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

func TestRunReviewAppLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := modelPatch()
	pending := comments.Comment{Repository: p.Root, Body: "pending"}
	if _, err := store.AddComment(pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := startApplication(ctx, p, store, func(
		received context.Context,
		kind patch.Kind,
	) (patch.Patch, error) {
		testutil.RequireValuesEqual(
			t,
			received,
			ctx,
			"context=%v kind=%v",
			received,
			kind,
		)
		testutil.RequireValuesEqual(
			t,
			kind,
			p.Kind,
			"context=%v kind=%v",
			received,
			kind,
		)

		return p, nil
	}, func() size {
		return size{
			Width:  120,
			Height: 40,
		}
	}, func(m model, _ ...tea.ProgramOption) error {
		testutil.RequireValuesEqual(t, m.ctx, ctx, "model=%#v", m)
		testutil.RequireValuesEqual(t, m.width, 120, "model=%#v", m)
		testutil.RequireValuesEqual(t, m.height, 40, "model=%#v", m)
		testutil.RequireValuesEqual(t, len(m.comments.items), 1, "model=%#v", m)

		result := m.refreshChanges()()
		msg, ok := result.(refreshDiffMsg)
		testutil.RequireValuesEqual(t, ok, true, "refresh message=%T", result)
		testutil.RequireValuesEqual(t, msg.err, nil, "%s", msg.err)

		return sentinel
	})
	testutil.RequireValuesEqual(
		t,
		errors.Is(err, sentinel),
		true,
		"error=%v",
		err,
	)
}

func TestRunReviewAppStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("HOME", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			config, err := os.UserConfigDir()
			testutil.RequireNoError(t, err)
			testutil.RequireNoError(t, os.MkdirAll(config, 0700))

			store := comments.Store{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
			prepareStartupFailure(t, failure, config, store.Path)
			err = startApplication(
				context.Background(),
				modelPatch(),
				store,
				nil,
				func() size { return defaultSize },
				func(
					model,
					...tea.ProgramOption,
				) error {
					t.Fatal("program started after startup failure")
					return nil
				},
			)
			testutil.RequireValuesEqual(t, err != nil, true, "%s",
				"expected startup error",
			)
		})
	}
}

func TestRunReviewAppContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := startApplication(ctx, modelPatch(), comments.Store{
		Path: filepath.Join(t.TempDir(), "comments.db"),
	}, nil, func() size { return defaultSize }, func(
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
	testutil.RequireValuesEqual(t, errors.Is(err, tea.ErrProgramKilled), true,
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
			got := readTerminalSize(probe.getSize)
			testutil.RequireValuesEqual(t, got, tc.want, "size=%v want=%v", got,
				tc.want,
			)
			testutil.RequireValuesEqual(
				t,
				probe.calls,
				tc.calls,
				"calls=%d want=%d",
				probe.calls,
				tc.calls,
			)
		})
	}
}

func prepareStartupFailure(t *testing.T, failure, config, database string) {
	t.Helper()
	if failure == "comments" {
		testutil.RequireNoError(t,
			os.WriteFile(database, []byte("invalid database"), 0600),
		)
	} else {
		testutil.RequireNoError(t,
			os.Mkdir(filepath.Join(config, "review-my-slop"), 0700),
		)
		testutil.RequireNoError(
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
	testutil.RequireValuesEqual(p.t, p.calls < len(descriptors), true,
		"unexpected terminal size call: %d", p.calls,
	)
	testutil.RequireValuesEqual(
		p.t,
		fd,
		descriptors[p.calls],
		"fd=%v want=%v",
		fd,
		descriptors[p.calls],
	)
	current := p.calls
	p.calls++
	if current == p.success {
		return 120, 40, nil
	}
	return 0, 0, errors.New("not a terminal")
}
