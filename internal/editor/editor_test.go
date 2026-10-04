package editor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCommandQuotesPathAndOmitsZeroLine(t *testing.T) {
	got := strings.Join(Command("editor", "/tmp/a file's.md", 0).Args, "\x00")
	want := "sh\x00-c\x00editor '/tmp/a file'\"'\"'s.md'"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
	got = strings.Join(Command("editor", "/tmp/a.md", 12).Args, "\x00")
	if got != "sh\x00-c\x00editor +12 '/tmp/a.md'" {
		t.Fatalf("command = %q", got)
	}
}

func TestPrepareFinishAndCloseOwnTemporaryFile(t *testing.T) {
	edit, err := Prepare("true", "initial")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(edit.path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(edit.path) != ".md" || info.Mode().Perm() != 0o600 {
		t.Fatalf("path=%q mode=%o", edit.path, info.Mode().Perm())
	}
	if err := os.WriteFile(edit.path, []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := edit.Finish(nil)
	if err != nil || text != "edited" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if _, err := os.Stat(edit.path); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
	if err := edit.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestFinishRemovesFileOnProcessFailure(t *testing.T) {
	edit, err := Prepare("false", "initial")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := edit.Finish(os.ErrPermission); err == nil {
		t.Fatal("process error was ignored")
	}
	if _, err := os.Stat(edit.path); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestFinishAndCloseCanRaceSafely(t *testing.T) {
	edit, err := Prepare("true", "initial")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSuffix(strings.TrimPrefix(edit.Command().Args[2], "true '"), "'")
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	wait.Go(func() {
		defer wait.Done()
		<-start
		_, _ = edit.Finish(nil)
	})
	wait.Go(func() {
		defer wait.Done()
		<-start
		_ = edit.Close()
	})
	close(start)
	wait.Wait()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}
