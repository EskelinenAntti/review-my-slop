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
)

func TestRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv(fixtureHOME, t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := modelPatch()
	{
		_, err := store.Add(
			comments.Comment{Repository: p.Root, Body: "pending"},
		)
		requireEqual(t, err == nil, true, valueFormat, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := startReview(

		ctx,

		p,

		store,

		func(received context.Context, kind patch.Kind) (
			patch.Patch,
			error,
		) {
			requireEqual(
				t,
				received == ctx && kind == p.Kind, true,
				"context=%v kind=%v",
				received,
				kind,
			)
			return p, nil
		},

		func() size { return size{Width: wideTestWidth, Height: forty} },

		func(
			m model,
			_ ...tea.ProgramOption,
		) error {
			requireEqual(

				t,

				m.ctx == ctx && m.width == wideTestWidth && m.height == forty &&

					len(m.comments.items) == one,

				true,

				"model=%#v",

				m,
			)
			{
				msg := requireType[refreshDiffMsg](t, m.loadRefresh()())
				requireEqual(t, msg.err == nil, true, valueFormat, msg.err)
			}
			return sentinel
		},
	)
	requireEqual(t, errors.Is(err, sentinel), true, "error=%v", err)
}

func TestRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(
			failure,
			func(t *testing.T) {
				assertStartupFailure(t, failure)
			},
		)
	}
}

func TestRunContextCancellation(t *testing.T) {
	t.Setenv(fixtureHOME, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := startReview(
		ctx,
		modelPatch(),
		comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")},
		nil,
		func() size {
			return defaultSize
		},
		func(
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
	)
	requireEqual(
		t,
		errors.Is(err, tea.ErrProgramKilled), true,
		"error=%v",
		err,
	)
}

func TestTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, success := range []int{zero, one, two} {
		assertTerminalSizeDiscovery(t, success)
	}
}

func assertStartupFailure(t *testing.T, failure string) {
	config := t.TempDir()
	t.Setenv(fixtureHOME, config)
	t.Setenv("XDG_CONFIG_HOME", config)
	config, err := os.UserConfigDir()
	requireEqual(t, err == nil, true, valueFormat, err)
	{
		err := os.MkdirAll(config, privateDirMode)
		requireEqual(t, err == nil, true, valueFormat, err)
	}
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if failure == "comments" {
		{
			err := os.WriteFile(
				store.Path,
				[]byte("invalid database"),
				privateFileMode,
			)
			requireEqual(t, err == nil, true, valueFormat, err)
		}
	} else {
		{
			err := os.Mkdir(
				filepath.Join(config, "review-my-slop"),
				privateDirMode,
			)
			requireEqual(t, err == nil, true, valueFormat, err)
		}
		{
			err := os.WriteFile(
				filepath.Join(config, "review-my-slop", "ui.json"),
				[]byte("invalid json"),
				privateFileMode,
			)
			requireEqual(t, err == nil, true, valueFormat, err)
		}
	}
	err = startReview(
		context.Background(),
		modelPatch(),
		store,
		nil,
		func() size {
			return defaultSize
		},
		func(
			model,
			...tea.ProgramOption,
		) error {
			t.Fatal("program started after startup failure")
			return nil
		},
	)
	requireEqual(t, err != nil, true, valueFormat, "expected startup error")
}

func assertTerminalSizeDiscovery(t *testing.T, success int) {
	calls := zero
	got := discoverTerminalSize(
		func(fd uintptr) (int, int, error) {
			expected := os.Stdin.Fd()
			if calls == one {
				expected = os.Stdout.Fd()
			}
			requireEqual(t, fd, expected, "fd=%v want=%v", fd, expected)
			current := calls
			calls++
			if current == success {
				return wideTestWidth, forty, nil
			}
			return zero, zero, errors.New("not a terminal")
		},
	)
	want := size{Width: wideTestWidth, Height: forty}
	if success == two {
		want = defaultSize
	}
	requireEqual(
		t,
		got == want && calls == min(success+one, two), true,
		"size=%v calls=%d",
		got,
		calls,
	)
}
