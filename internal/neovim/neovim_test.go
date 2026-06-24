package neovim

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

type memoryStore struct {
	comments []review.Comment
}

func (s *memoryStore) Save(comment review.Comment, repository string) (review.Comment, error) {
	comment.Repository = repository
	s.comments = append(s.comments, comment)
	return comment, nil
}

func TestHostInitializesAndSubmitsCodeComment(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	repository := initRepository(t)
	path := filepath.Join(repository, "main.go")
	if err := os.WriteFile(path, []byte("first()\nsecond()\nthird()\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v, err := nvim.NewChildProcess(
		nvim.ChildProcessContext(ctx),
		nvim.ChildProcessDir(repository),
		nvim.ChildProcessArgs("--embed", "--headless", "-u", "NONE", "-n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.Command("edit main.go"); err != nil {
		t.Fatal(err)
	}

	store := &memoryStore{}
	host := NewHost(ctx, store)
	p := plugin.New(v)
	host.Register(p)
	if err := v.Call("rpcrequest", nil, v.ChannelID(), MethodInitCodeComment, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	buffer, err := v.CurrentBuffer()
	if err != nil {
		t.Fatal(err)
	}
	name, err := v.BufferName(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "review-my-slop://comment/") {
		t.Fatalf("code comment buffer name = %q", name)
	}
	tabpages, err := v.Tabpages()
	if err != nil {
		t.Fatal(err)
	}
	if len(tabpages) != 2 {
		t.Fatalf("tab pages = %d, want source and full-screen code comment tabs", len(tabpages))
	}
	draft, err := v.BufferLines(buffer, 0, -1, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := joinLines(draft); got != "\n```suggestion\nfirst()\nsecond()\n```\n" {
		t.Fatalf("draft = %q", got)
	}
	if err := v.SetBufferLines(buffer, 0, -1, true, byteLines("Use clearer names.\n\n```suggestion\nfirst()\nsecond()\n```")); err != nil {
		t.Fatal(err)
	}
	if err := v.Call("rpcrequest", nil, v.ChannelID(), MethodSubmitCodeComment, int(buffer)); err != nil {
		t.Fatal(err)
	}

	if len(store.comments) != 1 {
		t.Fatalf("saved comments = %d", len(store.comments))
	}
	comment := store.comments[0]
	if comment.Repository != repository || comment.Anchor.FilePath != "main.go" {
		t.Fatalf("saved location = %#v", comment)
	}
	if comment.Anchor.NewStart != 1 || comment.Anchor.NewEnd != 2 {
		t.Fatalf("saved range = %#v", comment.Anchor)
	}
	if !slices.Equal(comment.Anchor.QuotedLines, []string{" first()", " second()"}) {
		t.Fatalf("quoted lines = %#v", comment.Anchor.QuotedLines)
	}
	if comment.Body != "Use clearer names." {
		t.Fatalf("body = %q", comment.Body)
	}
}

func TestRPCMethodNamesAreVersioned(t *testing.T) {
	for _, method := range []string{MethodInitCodeComment, MethodSubmitCodeComment, MethodDiscardCodeComment} {
		if !strings.HasPrefix(method, "review-my-slop/v1/code/comment/") {
			t.Fatalf("RPC method %q is not versioned", method)
		}
	}
}

func initRepository(t *testing.T) string {
	t.Helper()
	repository, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q")
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	return repository
}
