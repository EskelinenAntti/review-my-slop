package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func newTestAppRunModel(
	ctx context.Context,
	patch patches.Patch,
	comments []feedback.Comment,
	store commentStore,
	dimensions size,
) (reviewModel, error) {
	preferences, err := readTestUIPreferences()
	if err != nil {
		return reviewModel{}, err
	}
	layout := initialLayout{
		SideBySide:     preferences.SideBySide,
		SaveSideBySide: saveLayoutPreference,
		size:           dimensions,
	}
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	model := newTestBasicAppModel()
	model.ctx = ctx
	model.currentPatch = patch
	model.comments = commentState{items: comments}
	model.commentView = newTestCommentScreen(comments)
	model.width = size.Width
	model.height = size.Height
	model.saveLayout = layout.SaveSideBySide
	model.diffOptions = ui.Options{
		SideBySide: layout.SideBySide,
		Dark:       true,
	}
	model.kind = patch.Kind
	model.diffView = newTestDiffscreenView(patch, model.diffOptions)
	model.diffView.Resize(model.width, model.height)
	model.commentView.Resize(model.width, model.height)
	model.save = func(
		comment feedback.Comment,
		current patches.Patch,
	) (feedback.Comment, error) {
		return saveTestAppRunComment(store, comment, current)
	}
	model.delete = func(comment feedback.Comment, current patches.Patch) error {
		return store.Delete(current.Root, comment.ID)
	}
	model.load = func() ([]feedback.Comment, error) {
		return store.List(patch.Root)
	}
	return model, nil
}

func saveTestAppRunComment(
	store commentStore,
	comment feedback.Comment,
	current patches.Patch,
) (feedback.Comment, error) {
	comment.Repository = current.Root
	commentStore := store
	if comment.ID == "" {
		return commentStore.Add(comment)
	}
	if err := commentStore.Update(comment); err != nil {
		return feedback.Comment{}, err
	}
	return comment, nil
}

func testAppRunProgram(
	ctx context.Context,
	t *testing.T,
	width, height int,
	sentinel error,
) func(reviewModel, ...tea.ProgramOption) error {
	return func(model reviewModel, _ ...tea.ProgramOption) error {
		t.Helper()
		requireEqual(t, model.ctx, ctx, "model=%#v", model)
		requireEqual(t, model.width, width, "model=%#v", model)
		requireEqual(t, model.height, height, "model=%#v", model)
		requireEqual(t, len(model.comments.items), 1, "model=%#v", model)
		message, ok := testAppRefreshMessage(model).(refreshDiffMsg)
		requireEqual(t, ok, true, "refresh message=%T", message)
		requireEqual(t, message.err, nil, "%s", message.err)
		return sentinel
	}
}

func testAppRefreshMessage(model reviewModel) tea.Msg {
	if model.refresh == nil {
		return nil
	}
	kind := model.kind
	patch, err := model.refresh(kind)
	return refreshDiffMsg{patch: patch, kind: kind, err: err}
}

func runTestTeaProgram(model reviewModel, options ...tea.ProgramOption) error {
	options = append(
		options,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
	)
	_, err := tea.NewProgram(model, options...).Run()
	return err
}

func testStartupPatch() patches.Patch {
	return patches.Patch{Root: "/repo", Files: []patches.File{{
		DisplayPath: "main.go",
		OldPath:     "main.go",
		NewPath:     "main.go",
		Hunks: []patches.Hunk{{
			Header: "@@ -1,2 +1,2 @@",
			Lines: []patches.Line{
				{
					Kind:      patches.Context,
					Text:      "keep()",
					OldNumber: 1,
					NewNumber: 1,
				},
				{Kind: patches.Deletion, Text: "old()", OldNumber: 2},
				{Kind: patches.Addition, Text: "new()", NewNumber: 2},
			},
		}},
	}}}
}

func TestAppRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := feedback.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	patch := testStartupPatch()
	pending := feedback.Comment{Repository: patch.Root, Body: "pending"}
	if _, err := store.Add(pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	loaded, err := commentStore(store).List(patch.Root)
	requireNoError(t, err)
	dimensions := size{Width: 120, Height: 40}
	model, err := newTestAppRunModel(
		ctx, patch, loaded, commentStore(store), dimensions,
	)
	requireNoError(t, err)
	refresh := func(
		received context.Context,
		kind patches.Kind,
	) (patches.Patch, error) {
		requireEqual(t, received, ctx, "context=%v kind=%v", received, kind)
		requireEqual(t, kind, patch.Kind, "kind=%v", kind)
		return patch, nil
	}
	model.refresh = func(kind patches.Kind) (patches.Patch, error) {
		return refresh(ctx, kind)
	}
	err = testAppRunProgram(
		ctx,
		t,
		dimensions.Width,
		dimensions.Height,
		sentinel,
	)(model, tea.WithContext(ctx), tea.WithWindowSize(
		dimensions.Width, dimensions.Height,
	))
	requireEqual(t, errors.Is(err, sentinel), true, "error=%v", err)
}

func TestAppRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			err := testAppStartupFailure(t, failure)
			requireEqual(t, err != nil, true, "%s", "expected startup error")
		})
	}
}

func testAppStartupFailure(t *testing.T, failure string) error {
	t.Helper()
	store := invalidTestStartupStore(t, failure)
	ctx := context.Background()
	patch := testStartupPatch()
	pending, err := commentStore(store).List(patch.Root)
	if err != nil {
		return err
	}
	model, err := newTestAppRunModel(ctx, patch, pending, commentStore(
		store,
	), defaultSize)
	if err != nil {
		return err
	}
	return failTestAppStartupProgram(t, model)
}

func invalidTestStartupStore(t *testing.T, failure string) feedback.Store {
	t.Helper()
	config := t.TempDir()
	t.Setenv("HOME", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	config, err := os.UserConfigDir()
	requireNoError(t, err)
	requireNoError(t, os.MkdirAll(config, 0700))
	store := feedback.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if failure == "comments" {
		requireNoError(t, os.WriteFile(store.Path, []byte(
			"invalid database",
		), 0600))
		return store
	}
	settingsDir := filepath.Join(config, "review-my-slop")
	requireNoError(t, os.Mkdir(settingsDir, 0700))
	settingsPath := filepath.Join(settingsDir, "ui.json")
	requireNoError(t, os.WriteFile(settingsPath, []byte("invalid json"), 0600))
	return store
}

func failTestAppStartupProgram(
	t *testing.T,
	_ reviewModel,
	_ ...tea.ProgramOption,
) error {
	t.Helper()
	t.Fatal("program started after startup failure")
	return nil
}

func TestAppRunContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := func() error {
		var (
			currentPatch              = testStartupPatch()
			store        commentStore = feedback.Store{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
		)
		pending, err := store.List(currentPatch.Root)
		if err != nil {
			return err
		}
		dimensions := defaultSize
		m, err := newTestAppRunModel(
			ctx,
			currentPatch,
			pending,
			store,
			dimensions,
		)
		if err != nil {
			return err
		}
		return runTestTeaProgram(
			m,
			tea.WithContext(ctx),
			tea.WithWindowSize(dimensions.Width, dimensions.Height),
		)
	}()
	requireEqual(t, errors.Is(err, tea.ErrProgramKilled), true, "error=%v", err)
}

type appTerminalSizeCase struct {
	name           string
	success, calls int
	want           size
}

func TestAppTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, tc := range []appTerminalSizeCase{
		{"stdin", 0, 1, size{Width: 120, Height: 40}},
		{"stdout", 1, 2, size{Width: 120, Height: 40}},
		{"fallback", 2, 2, defaultSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertTerminalSizeDiscovery(t, tc)
		})
	}
}

func assertTerminalSizeDiscovery(t *testing.T, tc appTerminalSizeCase) {
	t.Helper()
	probe := appTerminalSizeProbe{t: t, success: tc.success}
	width, height, err := probe.size(os.Stdin.Fd())
	if err != nil {
		width, height, err = probe.size(os.Stdout.Fd())
	}
	got := defaultSize
	if err == nil {
		got = size{Width: width, Height: height}
	}
	requireEqual(t, got, tc.want, "size=%v want=%v", got, tc.want)
	requireEqual(t, probe.calls, tc.calls, "calls=%d want=%d",
		probe.calls, tc.calls,
	)
}

type appTerminalSizeProbe struct {
	t              *testing.T
	success, calls int
}

func (p *appTerminalSizeProbe) size(fd uintptr) (width, height int, err error) {
	p.t.Helper()
	descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
	requireEqual(p.t, p.calls < len(descriptors), true,
		"unexpected terminal size call: %d", p.calls)
	requireEqual(p.t, fd, descriptors[p.calls],
		"fd=%v want=%v", fd, descriptors[p.calls])
	current := p.calls
	p.calls++
	if current == p.success {
		return 120, 40, nil
	}
	return 0, 0, errors.New("not a terminal")
}
