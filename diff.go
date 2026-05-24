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
)

var changeColorRE = regexp.MustCompile(`\x1b\[[0-9;]*[39][12](?:;[0-9]*)?m`)
var leadingLineNoRE = regexp.MustCompile(`^\s*(\d+)\s`)
var anyLineNoRE = regexp.MustCompile(`(?:^|\s)(\d+)\s`)
var hunkRE = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

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
		refs, lines, files, err := loadDiff(args, source)
		ch <- diffResult{Source: source, Refs: refs, Lines: lines, Files: files, Err: err}
	}()
	return ch
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
		s.changedLines = nil
		s.localAvailable = false
		s.message = result.Err.Error()
		return
	}
	s.localAvailable = len(result.Refs) > 0
	if s.source != sourceLocal {
		return
	}
	s.lines = result.Lines
	s.files = result.Files
	s.changedLines = buildChangedLines(result.Lines, result.Refs)
	s.cursor = 0
	s.top = 0
	if len(result.Refs) == 0 {
		s.message = ""
	}
}

func (s *reviewState) reloadSource() error {
	return s.reloadSourceAt(s.current())
}

func (s *reviewState) reloadSourceAt(ref lineRef) error {
	refs, lines, files, err := loadDiff(s.sourceArgs, s.source)
	if err != nil {
		return err
	}
	s.clearSelection()
	if s.source == sourceLocal {
		s.localAvailable = len(refs) > 0
	}
	s.lines = lines
	s.files = files
	s.changedLines = buildChangedLines(lines, refs)
	s.restoreCursor(ref)
	s.message = fmt.Sprintf("Showing %s changes.", sourceLabel(s.source))
	return nil
}

func loadDiff(args []string, source diffSource) ([]lineRef, []string, []fileHeader, error) {
	refs, err := changedLineRefs(args)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(refs) == 0 {
		return refs, []string{fmt.Sprintf("No %s changes.", sourceLabel(source))}, nil, nil
	}
	diff, err := prettyDiff(args)
	if err != nil {
		return nil, nil, nil, err
	}
	lines := splitLines(diffWithUntrackedFiles(args, diff))
	return refs, lines, buildFileHeaders(lines), nil
}

func sourceLabel(source diffSource) string {
	if source == sourceBranch {
		return "branch"
	}
	return "uncommitted"
}

func splitLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func buildChangedLines(lines []string, fallback []lineRef) []changedLine {
	changedLines := buildDifftasticChangedLines(lines, true)
	if len(changedLines) > 0 {
		return changedLines
	}
	changedLines = buildDifftasticChangedLines(lines, false)
	if len(changedLines) > 0 {
		return changedLines
	}

	changedLines = make([]changedLine, 0, len(fallback))
	for i, ref := range fallback {
		row := changedLine{
			LineIndex: min(i, max(0, len(lines)-1)),
			Ref:       ref,
		}
		row.setSideRef(ref)
		changedLines = append(changedLines, row)
	}
	return changedLines
}

func buildDifftasticChangedLines(lines []string, changedOnly bool) []changedLine {
	var changedLines []changedLine
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
		if changedLine, ok := parseDifftasticRow(index, currentFile, plain, raw); ok {
			changedLine.LineIndex = lineIndex
			changedLines = append(changedLines, changedLine)
			index++
		}
	}

	return changedLines
}

type fileHeader struct {
	Line int
	File string
	Text string
}

func buildFileHeaders(lines []string) []fileHeader {
	var files []fileHeader
	for lineIndex, raw := range lines {
		plain := strings.TrimRight(ansi.Strip(raw), "\r")
		file, ok := parseDifftasticHeader(plain)
		if !ok {
			continue
		}
		files = append(files, fileHeader{Line: lineIndex, File: file, Text: raw})
	}
	return files
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

func parseDifftasticRow(index int, file, line, raw string) (changedLine, bool) {
	if strings.Contains(line, "│") {
		return changedLine{}, false
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

	var row changedLine
	if leftLine > 0 {
		row.setSideRef(lineRef{Index: index, File: file, Line: leftLine, Side: "old", Content: content})
	}
	if rightLine > 0 {
		row.setSideRef(lineRef{Index: index, File: file, Line: rightLine, Side: "new", Content: content})
	}
	row.Split = split
	if row.Right != nil {
		row.Ref = *row.Right
	} else if row.Left != nil {
		row.Ref = *row.Left
	}
	if row.Left == nil && row.Right == nil {
		return changedLine{}, false
	}
	return row, true
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

func changedLineRefs(args []string) ([]lineRef, error) {
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
