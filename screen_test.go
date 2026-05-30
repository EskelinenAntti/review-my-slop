package slop

import (
	"strings"
	"testing"

	"github.com/anttieskelinen/review-my-slop/internal/ansi"
)

type testCell struct {
	ch      byte
	inverse bool
	dim     bool
	fg      string
}

type testScreen struct {
	rows  int
	cols  int
	row   int
	col   int
	cells [][]testCell
	style testCell
}

func newTestScreen(rows, cols int) *testScreen {
	s := &testScreen{
		rows: rows,
		cols: cols,
	}
	s.cells = make([][]testCell, rows)
	for row := range rows {
		s.cells[row] = make([]testCell, cols)
		for col := range cols {
			s.cells[row][col].ch = ' '
		}
	}
	return s
}

func parseTestScreen(t *testing.T, rows, cols int, stream string) *testScreen {
	t.Helper()

	s := newTestScreen(rows, cols)
	for i := 0; i < len(stream); {
		switch stream[i] {
		case '\x1b':
			end := ansi.End(stream, i)
			if end < 0 {
				t.Fatalf("unterminated escape sequence at byte %d in %q", i, stream[i:])
			}
			s.applyEscape(stream[i:end])
			i = end
		case '\r':
			s.col = 0
			i++
		case '\n':
			if s.row < s.rows-1 {
				s.row++
			}
			i++
		default:
			s.put(stream[i])
			i++
		}
	}
	return s
}

func (s *testScreen) applyEscape(seq string) {
	switch {
	case seq == "\x1b[H":
		s.row = 0
		s.col = 0
	case seq == "\x1b[2J":
		for row := range s.rows {
			for col := range s.cols {
				s.cells[row][col] = testCell{ch: ' '}
			}
		}
		s.row = 0
		s.col = 0
	case seq == "\x1b[K":
		if s.row >= s.rows {
			return
		}
		for col := s.col; col < s.cols; col++ {
			s.cells[s.row][col] = testCell{ch: ' '}
		}
	case strings.HasSuffix(seq, "m"):
		s.applySGR(seq)
	}
}

func (s *testScreen) applySGR(seq string) {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if params == "" {
		params = "0"
	}
	for param := range strings.SplitSeq(params, ";") {
		switch param {
		case "0":
			s.style = testCell{}
		case "2":
			s.style.dim = true
		case "7":
			s.style.inverse = true
		case "31", "91":
			s.style.fg = "red"
		case "32", "92":
			s.style.fg = "green"
		}
	}
}

func (s *testScreen) put(ch byte) {
	if s.row >= s.rows || s.col >= s.cols {
		return
	}
	cell := s.style
	cell.ch = ch
	s.cells[s.row][s.col] = cell
	s.col++
}

func (s *testScreen) line(row int) string {
	var b strings.Builder
	for _, cell := range s.cells[row] {
		b.WriteByte(cell.ch)
	}
	return b.String()
}

func (s *testScreen) trimmedLine(row int) string {
	return strings.TrimRight(s.line(row), " ")
}

func (s *testScreen) text() string {
	var lines []string
	for row := range s.rows {
		lines = append(lines, s.trimmedLine(row))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (s *testScreen) inverseRange(row int) (int, int, bool) {
	start := -1
	end := -1
	for col, cell := range s.cells[row] {
		if cell.inverse {
			if start < 0 {
				start = col
			}
			end = col + 1
		}
	}
	return start, end, start >= 0
}

func renderScreen(t *testing.T, state *reviewState, rows, cols int) *testScreen {
	t.Helper()

	var out strings.Builder
	render(&out, state, rows, cols)
	return parseTestScreen(t, rows, cols, out.String())
}

func statusRow(rows int) int {
	if rows < 8 {
		rows = 8
	}
	return rows - 2
}

func helpRow(rows int) int {
	return statusRow(rows) + 1
}

func TestRenderScreenShowsCursorOnSelectedSideOnly(t *testing.T) {
	left := lineRef{File: "a.go", Line: 10, Side: "old", Content: "removed"}
	right := lineRef{File: "a.go", Line: 10, Side: "new", Content: "added"}
	state := &reviewState{
		lines: []string{
			"\x1b[91m 10 removed\x1b[0m       \x1b[92m 10 added\x1b[0m",
		},
		changedLines: []changedLine{{
			LineIndex: 0,
			Ref:       right,
			Left:      &left,
			Right:     &right,
			Split:     18,
		}},
	}

	screen := renderScreen(t, state, 8, 40)
	start, end, ok := screen.inverseRange(0)
	if !ok {
		t.Fatal("expected selected row to contain inverse cells")
	}
	if start != 18 || end != 40 {
		t.Fatalf("inverse range = %d-%d, want 18-40; line = %q", start, end, screen.line(0))
	}
	for col := range 18 {
		if screen.cells[0][col].inverse {
			t.Fatalf("left side column %d was inverse; line = %q", col, screen.line(0))
		}
	}
}

func TestRenderScreenKeepsCursorAcrossZeroInSelectedLine(t *testing.T) {
	lines := []string{
		"\x1b[1minternal/github/github.go\x1b[0m --- Go",
		"\x1b[92;1m 283 \x1b[0mif len(response.Data) == 0 {",
	}
	state := &reviewState{
		lines:        lines,
		changedLines: buildChangedLines(lines, nil),
	}
	if len(state.changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(state.changedLines))
	}

	screen := renderScreen(t, state, 8, 80)
	row := state.changedLines[0].LineIndex
	zeroCol := strings.Index(screen.line(row), "0")
	if zeroCol < 0 {
		t.Fatalf("selected line did not contain zero: %q", screen.line(row))
	}
	if !screen.cells[row][zeroCol].inverse {
		t.Fatalf("zero column was not inside cursor highlight; line = %q", screen.line(row))
	}
	start, end, ok := screen.inverseRange(row)
	if !ok {
		t.Fatalf("selected row had no inverse cells: %q", screen.line(row))
	}
	if start >= zeroCol || end <= zeroCol {
		t.Fatalf("inverse range = %d-%d, zero column = %d; line = %q", start, end, zeroCol, screen.line(row))
	}
}

func TestRenderScreenKeepsRightLineNumberColorWhenOldSideSelected(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[91m 1387 old name\x1b[0m       \x1b[92m 537 new name\x1b[0m",
	}
	state := &reviewState{
		lines:        lines,
		changedLines: buildChangedLines(lines, nil),
	}
	if len(state.changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(state.changedLines))
	}
	state.selectSide("old")

	screen := renderScreen(t, state, 8, 80)
	row := state.changedLines[0].LineIndex
	rightLineCol := strings.Index(screen.line(row), "537")
	if rightLineCol < 0 {
		t.Fatalf("right line number not found in rendered line: %q", screen.line(row))
	}
	if screen.cells[row][rightLineCol].inverse {
		t.Fatalf("right line number was inside old-side cursor highlight: %q", screen.line(row))
	}
	if screen.cells[row][rightLineCol].fg != "green" {
		t.Fatalf("right line number fg = %q, want green; line = %q", screen.cells[row][rightLineCol].fg, screen.line(row))
	}
}

func TestRenderScreenCanSelectAddedLineContainingTripleDash(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[92;1m 561 \x1b[0mbuf.WriteString(\" --- Text\\n\")",
	}
	state := &reviewState{
		lines:        lines,
		changedLines: buildChangedLines(lines, nil),
	}
	if len(state.changedLines) != 1 {
		t.Fatalf("expected one selectable row, got %d", len(state.changedLines))
	}

	screen := renderScreen(t, state, 8, 100)
	row := state.changedLines[0].LineIndex
	if !strings.Contains(screen.line(row), " --- Text") {
		t.Fatalf("rendered line missing triple-dash content: %q", screen.line(row))
	}
	if _, _, ok := screen.inverseRange(row); !ok {
		t.Fatalf("added line containing triple dash was not highlighted: %q", screen.line(row))
	}
}

func TestRenderScreenClearsStaleContentBetweenFrames(t *testing.T) {
	state := &reviewState{
		lines: []string{
			"short",
		},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "a.go", Line: 1, Side: "new", Content: "short"}),
		},
		message: "ok",
	}

	var out strings.Builder
	out.WriteString("leftover text that should disappear")
	render(&out, state, 8, 40)

	screen := parseTestScreen(t, 8, 40, out.String())
	if strings.Contains(screen.line(0), "leftover") {
		t.Fatalf("stale content remained on first row: %q", screen.line(0))
	}
	if got := strings.TrimRight(screen.line(1), " "); got != "" {
		t.Fatalf("row after short line = %q, want cleared blank row", got)
	}
}

func TestRenderScreenReviewModeTextSnapshot(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Number: 4},
		draft:  reviewDraft{Active: true},
		lines: []string{
			"README.md --- Text",
			"32 - `s` opens `$VISUAL` or `$EDITOR`",
			"33 - `A` approves, `C` comments, and `R` requests changes",
		},
		changedLines: []changedLine{
			{LineIndex: 1, Ref: lineRef{File: "README.md", Line: 32, Side: "new", Content: "`s` opens `$VISUAL` or `$EDITOR`"}},
			{LineIndex: 2, Ref: lineRef{File: "README.md", Line: 33, Side: "new", Content: "`A` approves, `C` comments, and `R` requests changes"}},
		},
		cursor: 1,
	}
	state.changedLines[0].setSideRef(state.changedLines[0].Ref)
	state.changedLines[1].setSideRef(state.changedLines[1].Ref)

	screen := renderScreen(t, state, 8, 160)
	got := screen.text()
	want := strings.TrimRight(`README.md --- Text
32 - `+"`s` opens `$VISUAL` or `$EDITOR`"+`
33 - `+"`A` approves, `C` comments, and `R` requests changes"+`




 h/j/k/l move  v select  c add comment  s add suggestion  A approve  C comment  R request changes  D delete draft  e open  o PR  r reload  q quit`, "\n")
	if got != want {
		t.Fatalf("screen text mismatch\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestRenderScreenShowsStickyFileHeader(t *testing.T) {
	state := &reviewState{
		top: 2,
		lines: []string{
			"\x1b[1ma.go\x1b[0m --- Go",
			" 1 first",
			" 2 second",
		},
		files: []fileHeader{{Line: 0, File: "a.go", Text: "\x1b[1ma.go\x1b[0m --- Go"}},
	}

	screen := renderScreen(t, state, 8, 40)
	if got := screen.trimmedLine(0); got != "a.go --- Go" {
		t.Fatalf("sticky header = %q, want file header", got)
	}
}

func TestRenderScreenKeepsSelectedRowBelowStickyFileHeader(t *testing.T) {
	state := &reviewState{
		top: 2,
		lines: []string{
			"a.go --- Go",
			" 1 first",
			" 2 selected",
			" 3 third",
		},
		files: []fileHeader{{Line: 0, File: "a.go", Text: "a.go --- Go"}},
		changedLines: []changedLine{
			{LineIndex: 2, Ref: lineRef{File: "a.go", Line: 2, Side: "new", Content: "selected"}},
		},
	}
	state.changedLines[0].setSideRef(state.changedLines[0].Ref)

	screen := renderScreen(t, state, 8, 40)
	if got := screen.trimmedLine(0); got != "a.go --- Go" {
		t.Fatalf("sticky header = %q, want file header", got)
	}
	if got := screen.trimmedLine(1); got != " 2 selected" {
		t.Fatalf("selected row = %q, want visible below sticky header", got)
	}
	if _, _, ok := screen.inverseRange(1); !ok {
		t.Fatalf("selected row was not highlighted below sticky header:\n%s", screen.text())
	}
}

func TestRenderScreenOmitsStatusRow(t *testing.T) {
	longContent := "`p` submits the pending review, opening `$VISUAL` or `$EDITOR` for an optional review summary"
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Number: 4},
		draft:  reviewDraft{Active: true, Count: 2},
		lines: []string{
			"README.md --- Text",
			"33 - " + longContent,
		},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new", Content: longContent}),
		},
	}

	screen := renderScreen(t, state, 8, 100)
	got := screen.text()
	for _, removed := range []string{"Draft review: 2 comments", "PR #4", "README.md:33"} {
		if strings.Contains(got, removed) {
			t.Fatalf("screen included removed status text %q:\n%s", removed, got)
		}
	}
}

func TestRenderScreenHelpShowsPRCheckPending(t *testing.T) {
	state := &reviewState{
		source:     sourceBranch,
		prChecking: true,
		lines:      []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 80)
	help := screen.trimmedLine(helpRow(8))
	if !strings.Contains(help, "checking PR") {
		t.Fatalf("help = %q, want PR checking state", help)
	}
}

func TestRenderScreenOmitsCompletedNoPRStatus(t *testing.T) {
	state := &reviewState{
		lines: []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 80)
	got := screen.text()
	if strings.Contains(got, "no PR") || strings.Contains(got, "checking PR") {
		t.Fatalf("screen included removed PR status:\n%s", got)
	}
}

func TestRenderScreenHelpReflectsReviewMode(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Number: 4},
		draft:  reviewDraft{Active: true},
		lines:  []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 120)
	help := screen.trimmedLine(helpRow(8))
	if strings.Contains(help, "P start review") {
		t.Fatalf("active review help should not show start-review action: %q", help)
	}
	if strings.Contains(help, "P submit review") {
		t.Fatalf("active review help should not show P submit action: %q", help)
	}
	for _, want := range []string{"A approve", "C comment", "R request changes", "D delete draft"} {
		if !strings.Contains(help, want) {
			t.Fatalf("active review help = %q, want %q", help, want)
		}
	}
}

func TestRenderScreenHelpHidesOwnPRDecisionActions(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Number: 4, Author: "octo", Viewer: "octo"},
		draft:  reviewDraft{Active: true},
		lines:  []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 120)
	help := screen.trimmedLine(helpRow(8))
	if strings.Contains(help, "A approve") || strings.Contains(help, "R request changes") {
		t.Fatalf("own PR help = %q, want no approve/request-changes actions", help)
	}
	for _, want := range []string{"C comment", "D delete draft"} {
		if !strings.Contains(help, want) {
			t.Fatalf("own PR help = %q, want %q", help, want)
		}
	}
}

func TestRenderScreenHelpReflectsPRStatus(t *testing.T) {
	state := &reviewState{
		source:     sourceBranch,
		prChecking: true,
		lines:      []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 100)
	help := screen.trimmedLine(helpRow(8))
	if strings.Contains(help, "P start review") || strings.Contains(help, "c comment") {
		t.Fatalf("checking help should not show PR actions: %q", help)
	}
	if !strings.Contains(help, "checking PR") {
		t.Fatalf("checking help = %q, want checking PR hint", help)
	}

	state.prChecking = false
	screen = renderScreen(t, state, 8, 100)
	help = screen.trimmedLine(helpRow(8))
	if strings.Contains(help, "P start review") || strings.Contains(help, "c comment") {
		t.Fatalf("no-PR help = %q, want no PR actions", help)
	}

	state.pr = &prContext{Number: 4}
	screen = renderScreen(t, state, 8, 100)
	help = screen.trimmedLine(helpRow(8))
	if !strings.Contains(help, "P start review") || !strings.Contains(help, "o PR") || strings.Contains(help, "c comment") || strings.Contains(help, "s suggest") {
		t.Fatalf("PR help = %q, want start-review and open-PR without comment actions", help)
	}
}

func TestRenderScreenHelpDoesNotShowDiffSwitch(t *testing.T) {
	state := &reviewState{
		source:          sourceLocal,
		branchAvailable: true,
		pr:              &prContext{Number: 4},
		lines:           []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 120)
	help := screen.trimmedLine(helpRow(8))
	if strings.Contains(help, "Tab diff") {
		t.Fatalf("help = %q, want no diff switch", help)
	}
}

func TestRenderScreenDetectedDraftShowsDraftActions(t *testing.T) {
	state := &reviewState{
		source: sourceBranch,
		pr:     &prContext{Number: 4},
		draft:  reviewDraft{Active: true, ID: "review-id", Count: 3},
		lines:  []string{"README.md --- Text"},
		changedLines: []changedLine{
			testChangedLine(lineRef{File: "README.md", Line: 33, Side: "new"}),
		},
	}

	screen := renderScreen(t, state, 8, 120)
	help := screen.trimmedLine(helpRow(8))
	if strings.Contains(screen.text(), "Draft review: 3 comments") {
		t.Fatalf("screen included removed draft status:\n%s", screen.text())
	}
	if strings.Contains(help, "P start review") || !strings.Contains(help, "D delete draft") {
		t.Fatalf("draft help = %q, want draft actions without start-review", help)
	}
}

func TestRenderScreenKeepsSelectedRowVisible(t *testing.T) {
	state := &reviewState{
		lines: []string{
			"line 1",
			"line 2",
			"line 3",
			"line 4 selected",
		},
		changedLines: []changedLine{
			{LineIndex: 3, Ref: lineRef{File: "a.go", Line: 4, Side: "new", Content: "line 4 selected"}},
		},
	}
	state.changedLines[0].setSideRef(state.changedLines[0].Ref)

	screen := renderScreen(t, state, 8, 80)
	bodyRows := statusRow(8)
	for row := range bodyRows {
		if _, _, ok := screen.inverseRange(row); ok && strings.Contains(screen.line(row), "line 4 selected") {
			return
		}
	}
	t.Fatalf("selected row was not visible in body:\n%s", screen.text())
}

func TestRenderScreenWrapsLongMessages(t *testing.T) {
	state := &reviewState{
		lines:   []string{"README.md --- Text", "body line"},
		message: `gh api graphql failed: submitPullRequestReview: Could not resolve to a PullRequestReview with the id "review-id"`,
	}

	screen := renderScreen(t, state, 8, 40)
	got := screen.text()
	for _, want := range []string{
		"gh api graphql failed:",
		"submitPullRequestReview:",
		"Could not",
		"resolve to a PullRequestReview",
		"review-id",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("screen missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(screen.trimmedLine(helpRow(8)), "q quit") {
		t.Fatalf("help row missing after wrapped message:\n%s", got)
	}
}
