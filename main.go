package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/ansi"
	"github.com/anttieskelinen/review-my-slop/internal/github"
)

var changeColorRE = regexp.MustCompile(`\x1b\[[0-9;]*[39][12](?:;[0-9]*)?m`)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rms:", err)
		os.Exit(1)
	}
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

func loadDiffAsync(args []string, source diffSource) <-chan diffResult {
	ch := make(chan diffResult, 1)
	go func() {
		refs, lines, err := loadDiff(args, source)
		ch <- diffResult{Source: source, Refs: refs, Lines: lines, Err: err}
	}()
	return ch
}

func detectReviewContextAsync() <-chan reviewContext {
	ch := make(chan reviewContext, 1)
	go func() {
		ch <- detectReviewContext()
	}()
	return ch
}

func detectReviewContext() reviewContext {
	pr := github.DetectPR()
	if pr == nil {
		return reviewContext{}
	}
	return reviewContext{
		PR:    pr,
		Draft: github.DetectPendingReview(pr),
	}
}

func (s *reviewState) receiveReviewContext(ch <-chan reviewContext) bool {
	if !s.prChecking {
		return false
	}
	select {
	case context := <-ch:
		s.applyReviewContext(context)
		return true
	default:
		return false
	}
}

func (s *reviewState) applyReviewContext(context reviewContext) {
	s.prChecking = false
	s.pr = context.PR
	s.draft = context.Draft
	s.branchAvailable = context.PR != nil
}

func (s *reviewState) applyDiffResult(result diffResult) {
	if result.Source != "" && result.Source != s.source {
		if result.Source == sourceLocal {
			s.localAvailable = len(result.Refs) > 0
		}
		return
	}
	if result.Err != nil {
		if s.source != sourceLocal {
			s.localAvailable = false
			return
		}
		s.lines = []string{"Unable to load diff."}
		s.selections = nil
		s.localAvailable = false
		s.message = result.Err.Error()
		return
	}
	s.localAvailable = len(result.Refs) > 0
	if s.source != sourceLocal {
		return
	}
	s.lines = result.Lines
	s.selections = buildSelections(result.Lines, result.Refs)
	s.cursor = 0
	s.top = 0
	if len(result.Refs) == 0 {
		s.message = ""
	}
}

func (s *reviewState) hasSelection() bool {
	return len(s.selections) > 0
}

func (s *reviewState) switchSource() error {
	if s.source == sourceLocal {
		if s.prChecking {
			return errors.New("Checking branch changes.")
		}
		if !s.branchAvailable {
			return errors.New("Branch changes are not available.")
		}
		return s.switchToSource(sourceBranch)
	}
	if !s.localAvailable {
		return errors.New("No uncommitted changes found.")
	}
	return s.switchToSource(sourceLocal)
}

func (s *reviewState) reloadSource() error {
	return s.switchToSource(s.source)
}

func (s *reviewState) switchToSource(source diffSource) error {
	args, err := s.argsForSource(source)
	if err != nil {
		return err
	}
	refs, lines, err := loadDiff(args, source)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return fmt.Errorf("No %s changes found.", sourceLabel(source))
	}
	s.clearSelection()
	s.source = source
	s.sourceArgs = args
	if source == sourceLocal {
		s.localAvailable = len(refs) > 0
	}
	s.lines = lines
	s.selections = buildSelections(lines, refs)
	s.cursor = 0
	s.top = 0
	s.message = fmt.Sprintf("Showing %s changes.", sourceLabel(source))
	return nil
}

func (s *reviewState) argsForSource(source diffSource) ([]string, error) {
	switch source {
	case sourceLocal:
		return s.args, nil
	case sourceBranch:
		if s.pr == nil {
			return nil, errors.New("No active GitHub PR found. Branch changes are not available.")
		}
		return []string{s.pr.Base + "...HEAD"}, nil
	default:
		return nil, fmt.Errorf("unknown diff source %q", source)
	}
}

func loadDiff(args []string, source diffSource) ([]lineRef, []string, error) {
	refs, err := changedLines(args)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return refs, []string{fmt.Sprintf("No %s changes.", sourceLabel(source))}, nil
	}
	diff, err := prettyDiff(args)
	if err != nil {
		return nil, nil, err
	}
	return refs, splitLines(diffWithUntrackedFiles(args, diff)), nil
}

func sourceLabel(source diffSource) string {
	if source == sourceBranch {
		return "branch"
	}
	return "uncommitted"
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

func (s *reviewState) current() lineRef {
	if !s.hasSelection() {
		return lineRef{}
	}
	return s.selections[s.cursor].Ref
}

func (s *reviewState) move(delta int) {
	next := s.cursor + delta
	s.moveTo(next)
}

func (s *reviewState) moveTo(next int) {
	if !s.hasSelection() {
		return
	}
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
	if !s.hasSelection() {
		s.top = 0
		return
	}
	bodyRows := max(1, rows-2)
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
	if !s.hasSelection() {
		s.message = "No changed line selected."
		return
	}
	if !s.canSelectRange() {
		s.message = "Multi-line selection is only available while reviewing branch changes."
		return
	}
	if s.selectionAnchor != nil {
		s.clearSelection()
		s.message = "Selection cleared."
		return
	}
	anchor := s.cursor
	s.selectionAnchor = &anchor
	s.message = "Selection started."
}

func (s *reviewState) canSelectRange() bool {
	return s.source == sourceBranch && s.draft.Active
}

func (s *reviewState) selectSide(side string) {
	if !s.hasSelection() {
		s.message = ""
		return
	}
	selection := &s.selections[s.cursor]
	ref := selection.sideRef(side)
	if ref == nil {
		s.message = ""
		return
	}

	previous := selection.Ref
	selection.Ref = *ref
	if s.selectionAnchor != nil && !s.selectionRangeValid() {
		selection.Ref = previous
		s.message = "Selection must stay within one file and side."
		return
	}
	s.message = ""
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
	if !s.hasSelection() {
		return reviewRange{}, errors.New("No changed line selected.")
	}
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
		return reviewRange{}, errors.New("Selection must stay within one file and side.")
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
		plain := strings.TrimRight(ansi.Strip(raw), "\r")
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
	if strings.Contains(line, "│") {
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
	if len(all) > 0 && hasGreen(raw) && !hasRed(raw) {
		leftLine = 0
		rightLine, _ = strconv.Atoi(all[0][1])
		split = allIndexes[0][2]
	} else if len(all) > 0 && hasRed(raw) && !hasGreen(raw) {
		leftLine, _ = strconv.Atoi(all[0][1])
	} else if len(all) > 1 {
		rightLine, _ = strconv.Atoi(all[len(all)-1][1])
		split = allIndexes[len(allIndexes)-1][0]
	} else if len(all) == 1 && leftLine == 0 {
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

func diffWithUntrackedFiles(args []string, diff []byte) []byte {
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
	if buf.Len() > 0 && !bytes.HasSuffix(diff, []byte("\n")) {
		buf.WriteByte('\n')
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(path)
		buf.WriteString(" --- Text\n")
		rendered, err := prettyUntrackedDiff(path)
		if err != nil || len(bytes.TrimSpace(rendered)) == 0 {
			rendered = plainUntrackedDiff(path)
		}
		buf.Write(rendered)
		if !bytes.HasSuffix(rendered, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func prettyUntrackedDiff(path string) ([]byte, error) {
	cmd := exec.Command("difft", "--color=always", "/dev/null", path)
	cmd.Env = append(os.Environ(), "DFT_COLOR=always")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

func plainUntrackedDiff(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var buf bytes.Buffer
	scanner := bufio.NewScanner(file)
	line := 1
	for scanner.Scan() {
		fmt.Fprintf(&buf, "\x1b[92;1m %d \x1b[0m%s\n", line, scanner.Text())
		line++
	}
	if line == 1 {
		buf.WriteString("\x1b[92;1m 1 \x1b[0m\n")
	}
	return buf.Bytes()
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
