package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentCancellationAndCreationFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string, context.CancelFunc)
	}{
		{
			"creation failure",
			func(t *testing.T, dir string, _ context.CancelFunc) {
				t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
			},
		},
		{
			"cancellation",
			func(
				t *testing.T,
				dir string,
				cancel context.CancelFunc,
			) {
				cancel()
				t.Setenv("TMPDIR", dir)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tc.setup(t, dir, cancel)
			adapter, complete := newCommentEditorWorkflow(
				ctx,
				"true",
				commentDraftInput{body: "body", anchor: comments.Anchor{}},
				func(
					body string,
					err error,
				) tea.Msg {
					testutil.RequireEqual(t, err != nil, true,
						fmt.Sprintf("body=%q err=%v", body, err),
					)
					testutil.RequireEqual(t, body, "",
						fmt.Sprintf("body=%q err=%v", body, err),
					)
					return nil
				},
			)
			complete(adapter.Run())
			files, err := os.ReadDir(dir)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(files), 0,
				fmt.Sprintf("files=%v", files),
			)
		})
	}
}

func TestMissingEditor(t *testing.T) {
	t.Setenv("EDITOR", " \t")
	cmd, err := EditComment(context.Background(), "", comments.Anchor{}, nil)
	testutil.RequireEqual(t, cmd == nil, true,
		fmt.Sprintf("cmd=%v err=%v", cmd, err),
	)
	testutil.RequireEqual(t, err != nil, true,
		fmt.Sprintf("cmd=%v err=%v", cmd, err),
	)
	cmd, err = OpenSource(context.Background(), "source", 2, nil)
	testutil.RequireEqual(t, cmd == nil, true,
		fmt.Sprintf("cmd=%v err=%v", cmd, err),
	)
	testutil.RequireEqual(t, err != nil, true,
		fmt.Sprintf("cmd=%v err=%v", cmd, err),
	)
}

type cancelOnWrite struct{ cancel context.CancelFunc }

func (w cancelOnWrite) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}

func TestRunningEditorCancellationCleansDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter, complete := newCommentEditorWorkflow(
		ctx,
		"printf started; exec sleep 30 #",
		commentDraftInput{body: "body", anchor: comments.Anchor{}},
		func(
			body string,
			err error,
		) tea.Msg {
			testutil.RequireEqual(t, err != nil, true,
				fmt.Sprintf("body=%q error=%v", body, err),
			)
			testutil.RequireEqual(t, body, "",
				fmt.Sprintf("body=%q error=%v", body, err),
			)

			return nil
		},
	)
	adapter.SetStdout(cancelOnWrite{cancel})
	complete(adapter.Run())
	files, err := os.ReadDir(dir)
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("files=%v error=%v", files, err),
	)
	testutil.RequireEqual(t, len(files), 0,
		fmt.Sprintf("files=%v error=%v", files, err),
	)
}
