package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
var changeColorRE = regexp.MustCompile(`\x1b\[[0-9;]*[39][12](?:;[0-9]*)?m`)

type lineRef struct {
	Index   int
	File    string
	Line    int
	Side    string
	Content string
}

type displaySelection struct {
	LineIndex int
	Ref       lineRef
	Left      *lineRef
	Right     *lineRef
	Split     int
}

type prContext struct {
	Number int    `json:"number"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Head   string `json:"head"`
	Base   string `json:"base"`
}

type reviewRange struct {
	Start lineRef
	End   lineRef
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rms:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	refs, err := changedLines(args)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		fmt.Fprintln(stdout, "No changed lines found.")
		return nil
	}

	diff, err := prettyDiff(args)
	if err != nil {
		return err
	}

	return reviewTUI(args, refs, diff, stdout)
}

func prettyDiff(args []string) ([]byte, error) {
	if _, err := exec.LookPath("difft"); err != nil {
		return nil, errors.New("difftastic is required: install `difft` and try again")
	}

	cmdArgs := append([]string{"-c", "diff.external=difft --color=always", "diff", "--ext-diff", "--color=always"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = append(os.Environ(), "DFT_COLOR=always")
	return cmd.CombinedOutput()
}

type terminalState struct {
	settings string
}

type reviewState struct {
	args            []string
	pr              *prContext
	selections      []displaySelection
	lines           []string
	cursor          int
	selectionAnchor *int
	top             int
	message         string
}

func reviewTUI(args []string, refs []lineRef, diff []byte, stdout io.Writer) error {
	lines := splitLines(diffWithUntrackedNotice(args, diff))
	state := &reviewState{
		args:       args,
		pr:         detectPRContext(),
		selections: buildSelections(lines, refs),
		lines:      lines,
	}
	if state.pr == nil {
		state.message = "No active GitHub PR found. Comments and suggestions are disabled."
	}

	term, err := enterTerminal()
	if err != nil {
		fmt.Fprintln(stdout, string(diffWithUntrackedNotice(args, diff)))
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Interactive review needs a terminal. Run `go run .` directly, then use j/k to move, e to open, q to quit.")
		return nil
	}
	defer term.restore()

	for {
		rows, cols := terminalSize()
		state.keepSelectionVisible(rows)
		render(stdout, state, rows, cols)

		key, err := readKey(os.Stdin)
		if err != nil {
			return err
		}

		switch key {
		case "q":
			return nil
		case "esc":
			if state.selectionAnchor != nil {
				state.clearSelection()
				state.message = ""
				continue
			}
			return nil
		case "j", "down":
			state.move(1)
		case "k", "up":
			state.move(-1)
		case "h", "left":
			state.selectSide("old")
		case "l", "right":
			state.selectSide("new")
		case "g":
			state.moveTo(0)
		case "G":
			state.moveTo(len(state.selections) - 1)
		case "ctrl-d", "pagedown":
			state.move(max(1, (rows-4)/2))
		case "ctrl-u", "pageup":
			state.move(-max(1, (rows-4)/2))
		case "e", "enter":
			state.clearSelection()
			ref := state.current()
			if ref.Side == "old" {
				state.message = fmt.Sprintf("Cannot open deleted line %s:%d", ref.File, ref.Line)
				continue
			}
			if err := withNormalTerminal(term, func() error {
				return openEditor(ref.File, ref.Line)
			}); err != nil {
				state.message = err.Error()
			} else {
				state.message = fmt.Sprintf("Opened %s:%d", ref.File, ref.Line)
			}
		case "r":
			state.clearSelection()
			diff, err := prettyDiff(state.args)
			if err != nil {
				state.message = err.Error()
				continue
			}
			refs, err := changedLines(state.args)
			if err != nil {
				state.message = err.Error()
				continue
			}
			if len(refs) == 0 {
				return nil
			}
			state.lines = splitLines(diffWithUntrackedNotice(state.args, diff))
			state.selections = buildSelections(state.lines, refs)
			if state.cursor >= len(state.selections) {
				state.cursor = len(state.selections) - 1
			}
			state.message = "Reloaded diff."
		case "v":
			state.toggleSelection()
		case "c":
			if err := state.reviewComment(term); err != nil {
				state.message = err.Error()
			}
		case "s":
			if err := state.reviewSuggestion(term); err != nil {
				state.message = err.Error()
			}
		}
	}
}

func enterTerminal() (*terminalState, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	state := &terminalState{settings: strings.TrimSpace(string(out))}
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	if err := raw.Run(); err != nil {
		return nil, err
	}
	fmt.Print("\x1b[?1049h\x1b[?25l")
	return state, nil
}

func (t *terminalState) restore() {
	fmt.Print("\x1b[?25h\x1b[?1049l")
	if t.settings == "" {
		return
	}
	cmd := exec.Command("stty", t.settings)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func withNormalTerminal(t *terminalState, fn func() error) error {
	fmt.Print("\x1b[?25h\x1b[?1049l")
	cmd := exec.Command("stty", t.settings)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
	err := fn()
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	_ = raw.Run()
	fmt.Print("\x1b[?1049h\x1b[?25l")
	return err
}

func detectPRContext() *prContext {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	cmd := exec.Command("gh", "pr", "view",
		"--json", "number,headRefOid,headRepository,headRepositoryOwner,baseRefOid",
		"--jq", `{"number": .number, "owner": .headRepositoryOwner.login, "repo": .headRepository.name, "head": .headRefOid, "base": .baseRefOid}`,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pr prContext
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil
	}
	if pr.Number == 0 || pr.Owner == "" || pr.Repo == "" || pr.Head == "" || pr.Base == "" {
		return nil
	}
	return &pr
}

func (s *reviewState) reviewComment(term *terminalState) error {
	return s.reviewWithBody(term, "")
}

func (s *reviewState) reviewSuggestion(term *terminalState) error {
	reviewRange, err := s.currentRange()
	if err != nil {
		return err
	}
	if reviewRange.End.Side != "new" {
		return errors.New("suggestions are only supported on the right side")
	}
	template, err := suggestionTemplate(reviewRange, s.pr)
	if err != nil {
		return err
	}
	return s.reviewWithBody(term, template)
}

func (s *reviewState) reviewWithBody(term *terminalState, template string) error {
	if s.pr == nil {
		return errors.New("no active GitHub PR found; cannot post review comments")
	}
	reviewRange, err := s.currentRange()
	if err != nil {
		return err
	}
	body, err := editReviewBody(term, template)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		s.message = "Cancelled empty review body."
		return nil
	}
	if err := postReviewComment(s.pr, reviewRange, body); err != nil {
		return err
	}
	s.clearSelection()
	if reviewRange.Start.Line == reviewRange.End.Line {
		s.message = fmt.Sprintf("Posted comment on %s:%d.", reviewRange.End.File, reviewRange.End.Line)
	} else {
		s.message = fmt.Sprintf("Posted comment on %s:%d-%d.", reviewRange.End.File, reviewRange.Start.Line, reviewRange.End.Line)
	}
	return nil
}

func editReviewBody(term *terminalState, template string) (string, error) {
	file, err := os.CreateTemp("", "rms-review-*.md")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.WriteString(template); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	if err := withNormalTerminal(term, func() error {
		return openEditorFile(path)
	}); err != nil {
		return "", err
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func suggestionTemplate(reviewRange reviewRange, pr *prContext) (string, error) {
	if pr == nil {
		return "", errors.New("no active GitHub PR found; cannot build suggestion")
	}
	lines, err := sourceLines(reviewRange, pr)
	if err != nil {
		return "", err
	}
	return "```suggestion\n" + strings.Join(lines, "\n") + "\n```\n", nil
}

func sourceLines(reviewRange reviewRange, pr *prContext) ([]string, error) {
	ref := pr.Head
	if reviewRange.End.Side == "old" {
		ref = pr.Base
	}
	out, err := gitShowFile(ref, reviewRange.End.File)
	if err != nil {
		return nil, err
	}
	lines := splitSourceLines(out)
	if reviewRange.Start.Line < 1 || reviewRange.End.Line > len(lines) {
		return nil, fmt.Errorf("cannot read %s:%d-%d from %s", reviewRange.End.File, reviewRange.Start.Line, reviewRange.End.Line, ref)
	}
	return lines[reviewRange.Start.Line-1 : reviewRange.End.Line], nil
}

func gitShowFile(ref, path string) (string, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("git show failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func splitSourceLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func postReviewComment(pr *prContext, reviewRange reviewRange, body string) error {
	data, err := json.Marshal(reviewCommentPayload(pr, reviewRange, body))
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", pr.Owner, pr.Repo, pr.Number)
	cmd := exec.Command("gh", "api", "-X", "POST", endpoint, "--input", "-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh api failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func reviewCommentPayload(pr *prContext, reviewRange reviewRange, body string) map[string]any {
	payload := map[string]any{
		"body":      body,
		"path":      reviewRange.End.File,
		"line":      reviewRange.End.Line,
		"side":      githubSide(reviewRange.End.Side),
		"commit_id": pr.Head,
	}
	if reviewRange.Start.Line != reviewRange.End.Line {
		payload["start_line"] = reviewRange.Start.Line
		payload["start_side"] = githubSide(reviewRange.Start.Side)
	}
	return payload
}

func githubSide(side string) string {
	if side == "old" {
		return "LEFT"
	}
	return "RIGHT"
}

func terminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 24, 80
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 24, 80
	}
	rows, err := strconv.Atoi(fields[0])
	if err != nil {
		rows = 24
	}
	cols, err := strconv.Atoi(fields[1])
	if err != nil {
		cols = 80
	}
	return rows, cols
}

func readKey(r io.Reader) (string, error) {
	var buf [1]byte
	if _, err := r.Read(buf[:]); err != nil {
		return "", err
	}
	switch buf[0] {
	case 3:
		return "q", nil
	case 4:
		return "ctrl-d", nil
	case 21:
		return "ctrl-u", nil
	case '\r', '\n':
		return "enter", nil
	case 27:
		var seq [2]byte
		if _, err := io.ReadFull(r, seq[:1]); err != nil {
			return "esc", nil
		}
		if seq[0] != '[' {
			return "esc", nil
		}
		if _, err := io.ReadFull(r, seq[1:]); err != nil {
			return "esc", nil
		}
		switch seq[1] {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		case 'C':
			return "right", nil
		case 'D':
			return "left", nil
		case '5':
			var discard [1]byte
			_, _ = io.ReadFull(r, discard[:])
			return "pageup", nil
		case '6':
			var discard [1]byte
			_, _ = io.ReadFull(r, discard[:])
			return "pagedown", nil
		}
		return "esc", nil
	default:
		return string(buf[0]), nil
	}
}

func render(w io.Writer, state *reviewState, rows, cols int) {
	if rows < 8 {
		rows = 8
	}
	if cols < 40 {
		cols = 40
	}
	bodyRows := rows - 4
	selectedLine := state.selectedDisplayLine()

	fmt.Fprint(w, "\x1b[H\x1b[2J")
	for row := range bodyRows {
		lineIndex := state.top + row
		if lineIndex >= len(state.lines) {
			fmt.Fprint(w, "\x1b[K\r\n")
			continue
		}
		line := truncateANSI(state.lines[lineIndex], cols)
		if selection, ok := state.displayLineSelection(lineIndex, cols); ok {
			fmt.Fprintf(w, "%s\x1b[K\r\n", highlightSelectionSide(line, cols, selection))
		} else if lineIndex == selectedLine {
			fmt.Fprintf(w, "%s\x1b[K\r\n", highlightSelectionSide(line, cols, state.selections[state.cursor]))
		} else {
			fmt.Fprintf(w, "%s\x1b[K\r\n", line)
		}
	}

	ref := state.current()
	marker := "+"
	if ref.Side == "old" {
		marker = "-"
	}
	mode := ""
	if state.selectionAnchor != nil {
		mode = " [SELECTING]"
	}
	pr := " no PR"
	if state.pr != nil {
		pr = fmt.Sprintf(" PR #%d", state.pr.Number)
	}
	status := fmt.Sprintf(" %d/%d%s%s %s %s:%d  %s", state.cursor+1, len(state.selections), mode, pr, marker, ref.File, ref.Line, strings.TrimSpace(ref.Content))
	help := " j/k move  h/l side  v select  c comment  s suggest  e/Enter open  g/G top/bottom  ^u/^d page  r reload  q quit "
	fmt.Fprintf(w, "\x1b[7m%s\x1b[0m\x1b[K\r\n", fit(status, cols))
	if state.message != "" {
		fmt.Fprintf(w, "%s\x1b[K\r\n", fit(" "+state.message, cols))
	} else {
		fmt.Fprint(w, "\x1b[K\r\n")
	}
	fmt.Fprintf(w, "\x1b[2m%s\x1b[0m\x1b[K", fit(help, cols))
}

func (s *reviewState) current() lineRef {
	return s.selections[s.cursor].Ref
}

func (s *reviewState) move(delta int) {
	next := s.cursor + delta
	s.moveTo(next)
}

func (s *reviewState) moveTo(next int) {
	if next < 0 {
		next = 0
	}
	if next >= len(s.selections) {
		next = len(s.selections) - 1
	}
	desiredSide := s.current().Side
	if s.selectionAnchor != nil {
		desiredSide = s.selections[*s.selectionAnchor].Ref.Side
	}
	if ref := s.selections[next].sideRef(desiredSide); ref != nil {
		s.selections[next].Ref = *ref
	}
	if s.selectionAnchor != nil && !s.canMoveSelectionTo(next) {
		return
	}
	s.cursor = next
	s.message = ""
}

func (s *reviewState) keepSelectionVisible(rows int) {
	bodyRows := max(1, rows-4)
	selectedLine := s.selectedDisplayLine()
	if selectedLine < 0 {
		return
	}
	if selectedLine < s.top {
		s.top = selectedLine
	}
	if selectedLine >= s.top+bodyRows {
		s.top = selectedLine - bodyRows + 1
	}
	if s.top < 0 {
		s.top = 0
	}
}

func (s *reviewState) selectedDisplayLine() int {
	return s.selections[s.cursor].LineIndex
}

func (s *reviewState) toggleSelection() {
	if s.selectionAnchor != nil {
		s.clearSelection()
		s.message = "Selection cleared."
		return
	}
	anchor := s.cursor
	s.selectionAnchor = &anchor
	ref := s.current()
	s.message = fmt.Sprintf("Selecting %s %s:%d", githubSide(ref.Side), ref.File, ref.Line)
}

func (s *reviewState) selectSide(side string) {
	selection := &s.selections[s.cursor]
	ref := selection.sideRef(side)
	if ref == nil {
		s.message = fmt.Sprintf("No %s side on this row.", sideLabel(side))
		return
	}

	previous := selection.Ref
	selection.Ref = *ref
	if s.selectionAnchor != nil && !s.selectionRangeValid() {
		selection.Ref = previous
		s.message = "Selection must stay within one file and side."
		return
	}
	s.message = fmt.Sprintf("Selected %s side.", sideLabel(side))
}

func (s *reviewState) clearSelection() {
	s.selectionAnchor = nil
}

func (s *reviewState) canMoveSelectionTo(cursor int) bool {
	if s.selectionAnchor == nil {
		return true
	}
	anchor := s.selections[*s.selectionAnchor].Ref
	next := s.selections[cursor].Ref
	return sameReviewTarget(anchor, next)
}

func (s *reviewState) currentRange() (reviewRange, error) {
	if s.selectionAnchor == nil {
		ref := s.current()
		return reviewRange{Start: ref, End: ref}, nil
	}

	startCursor, endCursor := *s.selectionAnchor, s.cursor
	if startCursor > endCursor {
		startCursor, endCursor = endCursor, startCursor
	}
	start := s.selections[startCursor].Ref
	end := s.selections[endCursor].Ref
	if !sameReviewTarget(start, end) {
		return reviewRange{}, errors.New("selection must stay within one file and side")
	}
	if start.Line > end.Line {
		start, end = end, start
	}
	return reviewRange{Start: start, End: end}, nil
}

func (s *reviewState) displayLineSelection(lineIndex, width int) (displaySelection, bool) {
	if s.selectionAnchor == nil {
		return displaySelection{}, false
	}
	start, end := *s.selectionAnchor, s.cursor
	if start > end {
		start, end = end, start
	}
	startLine := s.selections[start].LineIndex
	endLine := s.selections[end].LineIndex
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	if lineIndex < startLine || lineIndex > endLine {
		return displaySelection{}, false
	}
	for i := start; i <= end; i++ {
		if s.selections[i].LineIndex == lineIndex {
			return s.selections[i], true
		}
	}
	selection := s.selections[s.cursor]
	selection.Split = inferredSplit(s.lines[lineIndex], selection, width)
	return selection, true
}

func (s *reviewState) selectionRangeValid() bool {
	_, err := s.currentRange()
	return err == nil
}

func sameReviewTarget(a, b lineRef) bool {
	return a.File == b.File && a.Side == b.Side
}

func sideLabel(side string) string {
	if side == "old" {
		return "left"
	}
	return "right"
}

func splitLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func buildSelections(lines []string, fallback []lineRef) []displaySelection {
	selections := buildDifftasticSelections(lines, true)
	if len(selections) > 0 {
		return selections
	}
	selections = buildDifftasticSelections(lines, false)
	if len(selections) > 0 {
		return selections
	}

	selections = make([]displaySelection, 0, len(fallback))
	for i, ref := range fallback {
		selection := displaySelection{
			LineIndex: min(i, max(0, len(lines)-1)),
			Ref:       ref,
		}
		selection.setSideRef(ref)
		selections = append(selections, selection)
	}
	return selections
}

func buildDifftasticSelections(lines []string, changedOnly bool) []displaySelection {
	var selections []displaySelection
	currentFile := ""
	index := 1

	for lineIndex, raw := range lines {
		plain := strings.TrimRight(ansiRE.ReplaceAllString(raw, ""), "\r")
		if file, ok := parseDifftasticHeader(plain); ok {
			currentFile = file
			continue
		}
		if changedOnly && !changeColorRE.MatchString(raw) {
			continue
		}
		if currentFile == "" {
			continue
		}
		if selection, ok := parseDifftasticRow(index, currentFile, plain, raw); ok {
			selection.LineIndex = lineIndex
			selections = append(selections, selection)
			index++
		}
	}

	return selections
}

func parseDifftasticHeader(line string) (string, bool) {
	before, _, ok := strings.Cut(line, " --- ")
	if !ok {
		return "", false
	}
	file := strings.TrimSpace(before)
	if file == "" || strings.Contains(file, " ") {
		return "", false
	}
	return cleanDiffPath(file), true
}

var leadingLineNoRE = regexp.MustCompile(`^\s*(\d+)\s`)
var anyLineNoRE = regexp.MustCompile(`(?:^|\s)(\d+)\s`)

func parseDifftasticRow(index int, file, line, raw string) (displaySelection, bool) {
	if strings.Contains(line, "│") || strings.Contains(line, "---") {
		return displaySelection{}, false
	}

	content := strings.TrimSpace(line)
	var leftLine int
	if matches := leadingLineNoRE.FindStringSubmatch(line); len(matches) == 2 {
		leftLine, _ = strconv.Atoi(matches[1])
	}

	all := anyLineNoRE.FindAllStringSubmatch(line, -1)
	allIndexes := anyLineNoRE.FindAllStringSubmatchIndex(line, -1)
	rightLine := 0
	split := 0
	if len(all) > 1 {
		rightLine, _ = strconv.Atoi(all[len(all)-1][1])
		split = allIndexes[len(allIndexes)-1][2]
	}
	if len(all) == 1 && leftLine == 0 {
		lineNo, _ := strconv.Atoi(all[0][1])
		if hasGreen(raw) {
			rightLine = lineNo
			split = allIndexes[0][2]
		} else if hasRed(raw) {
			leftLine = lineNo
		}
	}

	var selection displaySelection
	if leftLine > 0 {
		selection.setSideRef(lineRef{Index: index, File: file, Line: leftLine, Side: "old", Content: content})
	}
	if rightLine > 0 {
		selection.setSideRef(lineRef{Index: index, File: file, Line: rightLine, Side: "new", Content: content})
	}
	selection.Split = split
	if selection.Right != nil {
		selection.Ref = *selection.Right
	} else if selection.Left != nil {
		selection.Ref = *selection.Left
	}
	if selection.Left == nil && selection.Right == nil {
		return displaySelection{}, false
	}
	return selection, true
}

func (s *displaySelection) setSideRef(ref lineRef) {
	refCopy := ref
	if ref.Side == "old" {
		s.Left = &refCopy
	} else {
		s.Right = &refCopy
	}
}

func (s displaySelection) sideRef(side string) *lineRef {
	if side == "old" {
		return s.Left
	}
	return s.Right
}

func hasGreen(s string) bool {
	return strings.Contains(s, "\x1b[32") || strings.Contains(s, "\x1b[92")
}

func hasRed(s string) bool {
	return strings.Contains(s, "\x1b[31") || strings.Contains(s, "\x1b[91")
}

func diffWithUntrackedNotice(args []string, diff []byte) []byte {
	if len(args) != 0 {
		return diff
	}
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return diff
	}
	var buf bytes.Buffer
	buf.Write(diff)
	if !bytes.HasSuffix(diff, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString("\nUntracked files:\n")
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		buf.WriteString("  ")
		buf.WriteString(scanner.Text())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func fit(s string, width int) string {
	plainLen := len(ansiRE.ReplaceAllString(s, ""))
	if plainLen <= width {
		return s
	}
	return truncateANSI(s, width)
}

func truncateANSI(s string, width int) string {
	if visibleLen(s) <= width {
		return s
	}
	if width <= 1 {
		return ""
	}

	var out strings.Builder
	visible := 0
	for i := 0; i < len(s) && visible < width-1; {
		if s[i] == '\x1b' {
			end := ansiEnd(s, i)
			if end > i {
				out.WriteString(s[i:end])
				i = end
				continue
			}
		}
		out.WriteByte(s[i])
		visible++
		i++
	}
	out.WriteByte(' ')
	out.WriteString("\x1b[0m")
	return out.String()
}

func highlightPlain(s string, width int) string {
	plain := ansiRE.ReplaceAllString(s, "")
	if len(plain) > width {
		plain = plain[:width]
	}
	if len(plain) < width {
		plain += strings.Repeat(" ", width-len(plain))
	}
	return "\x1b[7m" + plain + "\x1b[0m"
}

func highlightSelectionSide(s string, width int, selection displaySelection) string {
	start, end := selectionHighlightRange(selection, width)
	if start < 0 {
		start = 0
	}
	if end > width {
		end = width
	}
	if start >= end {
		return highlightPlain(s, width)
	}
	return highlightANSIRange(s, width, start, end)
}

func selectionHighlightRange(selection displaySelection, width int) (int, int) {
	if selection.Ref.Side == "old" {
		if selection.Split > 0 {
			return 0, selection.Split
		}
		return 0, width
	}
	if selection.Split > 0 {
		return selection.Split, width
	}
	return 0, width
}

func inferredSplit(line string, selection displaySelection, width int) int {
	if selection.Split > 0 {
		return selection.Split
	}
	plain := ansiRE.ReplaceAllString(line, "")
	if matches := anyLineNoRE.FindAllStringSubmatchIndex(plain, -1); len(matches) > 1 {
		return min(width, matches[len(matches)-1][2])
	}
	return 0
}

func highlightANSIRange(s string, width, start, end int) string {
	var out strings.Builder
	visible := 0
	inverse := false

	for i := 0; i < len(s) && visible < width; {
		if s[i] == '\x1b' {
			ansiEnd := ansiEnd(s, i)
			if ansiEnd > i {
				out.WriteString(s[i:ansiEnd])
				i = ansiEnd
				continue
			}
		}
		if !inverse && visible == start {
			out.WriteString("\x1b[7m")
			inverse = true
		}
		if inverse && visible == end {
			out.WriteString("\x1b[27m")
			inverse = false
		}
		out.WriteByte(s[i])
		visible++
		i++
	}
	for visible < width {
		if !inverse && visible == start {
			out.WriteString("\x1b[7m")
			inverse = true
		}
		if inverse && visible == end {
			out.WriteString("\x1b[27m")
			inverse = false
		}
		out.WriteByte(' ')
		visible++
	}
	if inverse {
		out.WriteString("\x1b[27m")
	}
	out.WriteString("\x1b[0m")
	return out.String()
}

func visibleLen(s string) int {
	visible := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			end := ansiEnd(s, i)
			if end > i {
				i = end
				continue
			}
		}
		visible++
		i++
	}
	return visible
}

func ansiEnd(s string, start int) int {
	if start+1 >= len(s) || s[start+1] != '[' {
		return -1
	}
	for i := start + 2; i < len(s); i++ {
		if s[i] >= '@' && s[i] <= '~' {
			return i + 1
		}
	}
	return -1
}

func changedLines(args []string) ([]lineRef, error) {
	cmdArgs := append([]string{"diff", "--no-ext-diff", "--unified=0"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	refs := parseDiff(out)
	if len(args) == 0 {
		untracked, err := untrackedLines(len(refs))
		if err != nil {
			return nil, err
		}
		refs = append(refs, untracked...)
	}
	return refs, nil
}

func untrackedLines(offset int) ([]lineRef, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("git ls-files failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var refs []lineRef
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		lineScanner := bufio.NewScanner(file)
		line := 1
		for lineScanner.Scan() {
			refs = append(refs, lineRef{
				Index:   offset + len(refs) + 1,
				File:    path,
				Line:    line,
				Side:    "new",
				Content: lineScanner.Text(),
			})
			line++
		}
		if line == 1 {
			refs = append(refs, lineRef{
				Index:   offset + len(refs) + 1,
				File:    path,
				Line:    1,
				Side:    "new",
				Content: "",
			})
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		if err := lineScanner.Err(); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return refs, nil
}

var hunkRE = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func parseDiff(diff []byte) []lineRef {
	var refs []lineRef
	scanner := bufio.NewScanner(bytes.NewReader(diff))
	oldFile, newFile := "", ""
	oldLine, newLine := 0, 0
	inHunk := false

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "--- "):
			oldFile = cleanDiffPath(strings.TrimPrefix(line, "--- "))
			inHunk = false
		case strings.HasPrefix(line, "+++ "):
			newFile = cleanDiffPath(strings.TrimPrefix(line, "+++ "))
			inHunk = false
		case strings.HasPrefix(line, "@@ "):
			matches := hunkRE.FindStringSubmatch(line)
			if len(matches) != 3 {
				inHunk = false
				continue
			}
			oldLine, _ = strconv.Atoi(matches[1])
			newLine, _ = strconv.Atoi(matches[2])
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+"):
			refs = append(refs, lineRef{
				Index:   len(refs) + 1,
				File:    newFile,
				Line:    newLine,
				Side:    "new",
				Content: strings.TrimPrefix(line, "+"),
			})
			newLine++
		case inHunk && strings.HasPrefix(line, "-"):
			refs = append(refs, lineRef{
				Index:   len(refs) + 1,
				File:    oldFile,
				Line:    oldLine,
				Side:    "old",
				Content: strings.TrimPrefix(line, "-"),
			})
			oldLine++
		case inHunk && strings.HasPrefix(line, " "):
			oldLine++
			newLine++
		}
	}

	return refs
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

func openEditor(file string, line int) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	var cmd *exec.Cmd
	lineArg := fmt.Sprintf("+%d", line)
	if strings.ContainsAny(editor, " \t") {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s %s", editor, shellQuote(lineArg), shellQuote(file)))
	} else {
		cmd = exec.Command(editor, lineArg, file)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openEditorFile(file string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	var cmd *exec.Cmd
	if strings.ContainsAny(editor, " \t") {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s", editor, shellQuote(file)))
	} else {
		cmd = exec.Command(editor, file)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
