package tui

import (
	"bytes"
	"io"
	"testing"
)

type testTerminal struct {
	rows      int
	cols      int
	entered   int
	exited    int
	suspended int
}

func (t *testTerminal) Enter() error {
	t.entered++
	return nil
}

func (t *testTerminal) Exit() {
	t.exited++
}

func (t *testTerminal) Suspend(fn func() error) error {
	t.suspended++
	return fn()
}

func (t *testTerminal) Size() (int, int) {
	return t.rows, t.cols
}

type testApp struct {
	keys []Key
}

func (a *testApp) Draw(w io.Writer, rows, cols int) {
	_, _ = w.Write([]byte("draw\n"))
}

func (a *testApp) Handle(key Key, term Terminal) (bool, error) {
	a.keys = append(a.keys, key)
	return key == "q", nil
}

func TestSessionRunsUntilAppQuits(t *testing.T) {
	term := &testTerminal{rows: 10, cols: 40}
	app := &testApp{}
	var out bytes.Buffer

	err := (Session{
		Input:    stringsReader("jq"),
		Output:   &out,
		Terminal: term,
	}).Run(app)

	if err != nil {
		t.Fatal(err)
	}
	if term.entered != 1 || term.exited != 1 {
		t.Fatalf("terminal enter/exit = %d/%d", term.entered, term.exited)
	}
	if got, want := app.keys, []Key{"j", "q"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("keys = %#v, want %#v", got, want)
	}
	if out.String() != "draw\ndraw\n" {
		t.Fatalf("draw output = %q", out.String())
	}
}

func stringsReader(s string) io.Reader {
	return bytes.NewBufferString(s)
}
