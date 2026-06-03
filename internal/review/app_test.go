package review

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/diffparse"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

type fakeRunner struct {
	calls     []runnerCall
	responses []runnerResponse
}

type runnerCall struct {
	name string
	args []string
	env  []string
}

type runnerResponse struct {
	out []byte
	err error
}

func (r *fakeRunner) Run(name string, args []string, env []string) ([]byte, error) {
	r.calls = append(r.calls, runnerCall{
		name: name,
		args: append([]string(nil), args...),
		env:  append([]string(nil), env...),
	})
	if len(r.responses) == 0 {
		return nil, nil
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response.out, response.err
}

type fakeParser struct {
	lines []diffparse.Line
	seen  [][]string
}

func (p *fakeParser) Parse(lines []string) []diffparse.Line {
	p.seen = append(p.seen, append([]string(nil), lines...))
	return append([]diffparse.Line(nil), p.lines...)
}

type fakeEditor struct {
	opened []diffparse.Location
}

func (e *fakeEditor) Open(file string, line int) error {
	e.opened = append(e.opened, diffparse.Location{File: file, Line: line})
	return nil
}

type fakeTerm struct {
	suspended int
}

func (t *fakeTerm) Enter() error {
	return nil
}

func (t *fakeTerm) Exit() {}

func (t *fakeTerm) Suspend(fn func() error) error {
	t.suspended++
	return fn()
}

func (t *fakeTerm) Size() (int, int) {
	return 5, 20
}

func TestLoaderShowsLocalChangesWhenWorktreeIsDirty(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{
		{out: []byte(" M a.go\n")},
		{},
		{out: []byte("one\r\ntwo\n")},
		{},
	}}
	parser := &fakeParser{lines: []diffparse.Line{{Text: "parsed"}}}
	loader := Loader{Runner: runner, Parser: parser}

	lines, err := loader.Load()

	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Text != "parsed" {
		t.Fatalf("lines = %#v", lines)
	}
	wantArgs := []string{"-c", "diff.external=difft --color=always", "diff", "--ext-diff", "--color=always", "HEAD"}
	if !reflect.DeepEqual(runner.calls[2].args, wantArgs) {
		t.Fatalf("git diff args = %#v, want %#v", runner.calls[2].args, wantArgs)
	}
	if !reflect.DeepEqual(parser.seen[0], []string{"one", "two"}) {
		t.Fatalf("parser input = %#v", parser.seen[0])
	}
}

func TestLoaderAppendsUntrackedFilesToLocalDiff(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{
		{out: []byte("?? new.go\n")},
		{},
		{out: []byte("tracked\n")},
		{out: []byte("new.go\n")},
		{out: []byte("rendered new\n")},
	}}
	parser := &fakeParser{lines: []diffparse.Line{{Text: "parsed"}}}
	loader := Loader{Runner: runner, Parser: parser}

	_, err := loader.Load()

	if err != nil {
		t.Fatal(err)
	}
	if runner.calls[4].name != "difft" || !reflect.DeepEqual(runner.calls[4].args, []string{"--color=always", "/dev/null", "new.go"}) {
		t.Fatalf("untracked render call = %#v", runner.calls[4])
	}
	if !reflect.DeepEqual(parser.seen[0], []string{"tracked", "", "new.go --- Text", "rendered new"}) {
		t.Fatalf("parser input = %#v", parser.seen[0])
	}
}

func TestLoaderDoesNotDuplicateUntrackedHeaderWhenDifftasticRendersOne(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{
		{out: []byte("?? internal/diffparse/difftastic.go\n")},
		{},
		{},
		{out: []byte("internal/diffparse/difftastic.go\n")},
		{out: []byte("internal/diffparse/difftastic.go --- Go\n\x1b[92m 1 package diffparse\x1b[0m\n")},
	}}
	parser := &fakeParser{lines: []diffparse.Line{{Text: "parsed"}}}
	loader := Loader{Runner: runner, Parser: parser}

	_, err := loader.Load()

	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/diffparse/difftastic.go --- Go",
		"\x1b[92m 1 package diffparse\x1b[0m",
	}
	if !reflect.DeepEqual(parser.seen[0], want) {
		t.Fatalf("parser input = %#v, want %#v", parser.seen[0], want)
	}
}

func TestLoaderShowsBranchDiffWhenWorktreeIsClean(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{
		{},
		{out: []byte("origin/main\n")},
		{},
		{out: []byte("branch\n")},
	}}
	parser := &fakeParser{lines: []diffparse.Line{{Text: "parsed"}}}
	loader := Loader{Runner: runner, Parser: parser}

	_, err := loader.Load()

	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"-c", "diff.external=difft --color=always", "diff", "--ext-diff", "--color=always", "origin/main...HEAD"}
	if !reflect.DeepEqual(runner.calls[3].args, wantArgs) {
		t.Fatalf("git diff args = %#v, want %#v", runner.calls[3].args, wantArgs)
	}
}

func TestLoaderSkipsMissingOriginHeadTarget(t *testing.T) {
	runner := &fakeRunner{responses: []runnerResponse{
		{},
		{out: []byte("origin/experiment\n")},
		{err: errors.New("missing origin/experiment")},
		{},
		{out: []byte("branch\n")},
	}}
	parser := &fakeParser{lines: []diffparse.Line{{Text: "parsed"}}}
	loader := Loader{Runner: runner, Parser: parser}

	_, err := loader.Load()

	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"-c", "diff.external=difft --color=always", "diff", "--ext-diff", "--color=always", "origin/main...HEAD"}
	if !reflect.DeepEqual(runner.calls[4].args, wantArgs) {
		t.Fatalf("git diff args = %#v, want %#v", runner.calls[4].args, wantArgs)
	}
}

func TestAppNavigationEditRefreshesAndRestoresCursorByLocation(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{
			{
				{Text: "a.go --- Go"},
				{Text: "line 10", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
				{Text: "line 20", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
			},
			{
				{Text: "a.go --- Go"},
				{Text: "line 20 moved", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
				{Text: "line 10 moved", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
			},
		},
	}
	editor := &fakeEditor{}
	app, err := New(Loader{Runner: loader, Parser: loader}, editor)
	if err != nil {
		t.Fatal(err)
	}
	term := &fakeTerm{}

	if _, err := app.Handle("j", term); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Handle("j", term); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Handle("e", term); err != nil {
		t.Fatal(err)
	}

	if want := []diffparse.Location{{File: "a.go", Line: 20}}; !reflect.DeepEqual(editor.opened, want) {
		t.Fatalf("opened = %#v, want %#v", editor.opened, want)
	}
	if term.suspended != 1 {
		t.Fatalf("suspended = %d, want 1", term.suspended)
	}
	if app.viewport.Cursor != 1 {
		t.Fatalf("cursor = %d, want restored line index 1", app.viewport.Cursor)
	}
}

func TestAppCursorSkipsHeaders(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{{
			{Text: "a.go --- Go"},
			{Text: "line 10", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
			{Text: "line 20", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
		}},
	}
	app, err := New(Loader{Runner: loader, Parser: loader}, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}

	if app.viewport.Cursor != 1 {
		t.Fatalf("initial cursor = %d, want first editable line 1", app.viewport.Cursor)
	}
	if _, err := app.Handle("k", &fakeTerm{}); err != nil {
		t.Fatal(err)
	}
	if app.viewport.Cursor != 1 {
		t.Fatalf("cursor after moving up = %d, want header skipped", app.viewport.Cursor)
	}
}

func TestAppDrawShowsHeaderBeforeInitialCursor(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{{
			{Text: "a.go --- Go", Header: true},
			{Text: "line 10", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
			{Text: "line 20", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
			{Text: "line 30", Location: diffparse.Location{File: "a.go", Line: 30}, Selectable: true, Editable: true},
		}},
	}
	app, err := New(Loader{Runner: loader, Parser: loader}, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer

	app.Draw(&out, 3, 40)

	if !bytes.Contains(out.Bytes(), []byte("a.go --- Go")) {
		t.Fatalf("draw output did not include header: %q", out.String())
	}
	if app.viewport.Top != 0 {
		t.Fatalf("top = %d, want header visible at top 0", app.viewport.Top)
	}
}

func TestAppDrawUsesStickyHeaderForTopmostFile(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{{
			{Text: "a.go --- Go", Header: true},
			{Text: "a line 1", Location: diffparse.Location{File: "a.go", Line: 1}, Selectable: true, Editable: true},
			{Text: "a line 2", Location: diffparse.Location{File: "a.go", Line: 2}, Selectable: true, Editable: true},
			{Text: "a line 3", Location: diffparse.Location{File: "a.go", Line: 3}, Selectable: true, Editable: true},
			{Text: "b.go --- Go", Header: true},
			{Text: "b line 1", Location: diffparse.Location{File: "b.go", Line: 1}, Selectable: true, Editable: true},
		}},
	}
	app, err := New(Loader{Runner: loader, Parser: loader}, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	app.viewport.Rows = 3
	app.viewport.Cursor = 3
	app.viewport.Top = 2
	var out bytes.Buffer

	app.Draw(&out, 3, 40)

	if !bytes.HasPrefix(out.Bytes(), []byte("\x1b[H\x1b[2Ja.go --- Go")) {
		t.Fatalf("draw output did not start with sticky header: %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("a line 2")) {
		t.Fatalf("draw output hid top content below sticky header: %q", out.String())
	}
}

func TestAppDrawKeepsCursorVisibleBelowStickyHeader(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{{
			{Text: "a.go --- Go", Header: true},
			{Text: "a line 1", Location: diffparse.Location{File: "a.go", Line: 1}, Selectable: true, Editable: true},
			{Text: "a line 2", Location: diffparse.Location{File: "a.go", Line: 2}, Selectable: true, Editable: true},
			{Text: "a line 3", Location: diffparse.Location{File: "a.go", Line: 3}, Selectable: true, Editable: true},
			{Text: "a line 4", Location: diffparse.Location{File: "a.go", Line: 4}, Selectable: true, Editable: true},
		}},
	}
	app, err := New(Loader{Runner: loader, Parser: loader}, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	app.viewport.Rows = 3
	app.viewport.Cursor = 4
	app.viewport.Top = 2
	var out bytes.Buffer

	app.Draw(&out, 3, 40)

	if app.viewport.Top != 3 {
		t.Fatalf("top = %d, want 3 so cursor is visible under sticky header", app.viewport.Top)
	}
	if !bytes.Contains(out.Bytes(), []byte("a line 4")) {
		t.Fatalf("draw output did not contain cursor row: %q", out.String())
	}
}

func TestAppCursorCanSelectDeletedRows(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{{
			{Text: "deleted.go --- Go"},
			{Text: "removed 1", Location: diffparse.Location{File: "deleted.go", Line: 1}, Selectable: true},
			{Text: "removed 2", Location: diffparse.Location{File: "deleted.go", Line: 2}, Selectable: true},
		}},
	}
	editor := &fakeEditor{}
	app, err := New(Loader{Runner: loader, Parser: loader}, editor)
	if err != nil {
		t.Fatal(err)
	}

	if app.viewport.Cursor != 1 {
		t.Fatalf("initial cursor = %d, want first deleted row", app.viewport.Cursor)
	}
	if _, err := app.Handle("j", &fakeTerm{}); err != nil {
		t.Fatal(err)
	}
	if app.viewport.Cursor != 2 {
		t.Fatalf("cursor = %d, want second deleted row", app.viewport.Cursor)
	}
	if _, err := app.Handle("e", &fakeTerm{}); err != nil {
		t.Fatal(err)
	}
	if len(editor.opened) != 0 {
		t.Fatalf("deleted row opened editor: %#v", editor.opened)
	}
}

func TestAppDrawHasNoMessagesOrHints(t *testing.T) {
	parser := &fakeParser{lines: []diffparse.Line{{Text: "only diff"}}}
	app, err := New(Loader{Runner: &fakeRunner{responses: []runnerResponse{{out: []byte(" M a.go\n")}, {}, {out: []byte("ignored")}, {}}}, Parser: parser}, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer

	app.Draw(&out, 3, 40)

	if bytes.Contains(out.Bytes(), []byte("j/k")) || bytes.Contains(out.Bytes(), []byte("quit")) || bytes.Contains(out.Bytes(), []byte("open")) {
		t.Fatalf("draw output contains hints: %q", out.String())
	}
}

func TestReviewSessionNavigatesEditsRefreshesAndQuits(t *testing.T) {
	loader := &sequenceLoader{
		loads: [][]diffparse.Line{
			{
				{Text: "a.go --- Go"},
				{Text: "line 10", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
				{Text: "line 20", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
			},
			{
				{Text: "a.go --- Go"},
				{Text: "line 20 changed", Location: diffparse.Location{File: "a.go", Line: 20}, Selectable: true, Editable: true},
				{Text: "line 10 changed", Location: diffparse.Location{File: "a.go", Line: 10}, Selectable: true, Editable: true},
			},
		},
	}
	editor := &fakeEditor{}
	app, err := New(Loader{Runner: loader, Parser: loader}, editor)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer

	err = (tui.Session{
		Input:    bytes.NewBufferString("jjeq"),
		Output:   &out,
		Terminal: &fakeTerm{},
	}).Run(app)

	if err != nil {
		t.Fatal(err)
	}
	if want := []diffparse.Location{{File: "a.go", Line: 20}}; !reflect.DeepEqual(editor.opened, want) {
		t.Fatalf("opened = %#v, want %#v", editor.opened, want)
	}
	if app.viewport.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", app.viewport.Cursor)
	}
	if !bytes.Contains(out.Bytes(), []byte("line 20 changed")) {
		t.Fatalf("output did not include refreshed diff: %q", out.String())
	}
}

type sequenceLoader struct {
	loads [][]diffparse.Line
	index int
}

func (l *sequenceLoader) Run(string, []string, []string) ([]byte, error) {
	return []byte(" M a.go"), nil
}

func (l *sequenceLoader) Parse([]string) []diffparse.Line {
	if l.index >= len(l.loads) {
		return l.loads[len(l.loads)-1]
	}
	lines := l.loads[l.index]
	l.index++
	return lines
}

func TestLoaderReturnsCommandOutputOnError(t *testing.T) {
	loader := Loader{Runner: &fakeRunner{responses: []runnerResponse{{out: []byte("fatal\n"), err: errors.New("exit 1")}}}, Parser: &fakeParser{}}

	_, err := loader.Load()

	if err == nil || err.Error() != "fatal" {
		t.Fatalf("err = %v, want fatal", err)
	}
}
