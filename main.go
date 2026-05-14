package main

import (
	"bufio"
	"bytes"
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
	args       []string
	selections []displaySelection
	lines      []string
	cursor     int
	top        int
	message    string
}

func reviewTUI(args []string, refs []lineRef, diff []byte, stdout io.Writer) error {
	lines := splitLines(diffWithUntrackedNotice(args, diff))
	state := &reviewState{
		args:       args,
		selections: buildSelections(lines, refs),
		lines:      lines,
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
		case "q", "esc":
			return nil
		case "j", "down":
			state.move(1)
		case "k", "up":
			state.move(-1)
		case "g":
			state.cursor = 0
		case "G":
			state.cursor = len(state.selections) - 1
		case "ctrl-d", "pagedown":
			state.move(max(1, (rows-4)/2))
		case "ctrl-u", "pageup":
			state.move(-max(1, (rows-4)/2))
		case "e", "enter":
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
		if lineIndex == selectedLine {
			fmt.Fprintf(w, "%s\x1b[K\r\n", highlightPlain(line, cols))
		} else {
			fmt.Fprintf(w, "%s\x1b[K\r\n", line)
		}
	}

	ref := state.current()
	marker := "+"
	if ref.Side == "old" {
		marker = "-"
	}
	status := fmt.Sprintf(" %d/%d %s %s:%d  %s", state.cursor+1, len(state.selections), marker, ref.File, ref.Line, strings.TrimSpace(ref.Content))
	help := " j/k move  e/Enter open  g/G top/bottom  ^u/^d page  r reload  q quit "
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
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(s.selections) {
		s.cursor = len(s.selections) - 1
	}
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
		selections = append(selections, displaySelection{
			LineIndex: min(i, max(0, len(lines)-1)),
			Ref:       ref,
		})
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
		if ref, ok := parseDifftasticRow(index, currentFile, plain, raw); ok {
			ref.Content = strings.TrimSpace(plain)
			selections = append(selections, displaySelection{
				LineIndex: lineIndex,
				Ref:       ref,
			})
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

func parseDifftasticRow(index int, file, line, raw string) (lineRef, bool) {
	if strings.Contains(line, "│") || strings.Contains(line, "---") {
		return lineRef{}, false
	}

	var leftLine int
	if matches := leadingLineNoRE.FindStringSubmatch(line); len(matches) == 2 {
		leftLine, _ = strconv.Atoi(matches[1])
	}

	all := anyLineNoRE.FindAllStringSubmatch(line, -1)
	rightLine := 0
	if len(all) > 1 {
		rightLine, _ = strconv.Atoi(all[len(all)-1][1])
	}
	if len(all) == 1 && leftLine == 0 {
		lineNo, _ := strconv.Atoi(all[0][1])
		if hasGreen(raw) {
			rightLine = lineNo
		} else if hasRed(raw) {
			leftLine = lineNo
		}
	}

	switch {
	case rightLine > 0:
		return lineRef{Index: index, File: file, Line: rightLine, Side: "new"}, true
	case leftLine > 0:
		return lineRef{Index: index, File: file, Line: leftLine, Side: "old"}, true
	default:
		return lineRef{}, false
	}
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
