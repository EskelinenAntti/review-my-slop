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
	"time"

	"github.com/anttieskelinen/review-my-slop/internal/ansi"
	"github.com/anttieskelinen/review-my-slop/internal/github"
	"github.com/anttieskelinen/review-my-slop/internal/keys"
)

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

type prContext = github.PR

type reviewRange struct {
	Start lineRef
	End   lineRef
}

type reviewDraft = github.Draft

type reviewContext struct {
	PR    *prContext
	Draft reviewDraft
}

type keyResult struct {
	Key string
	Err error
}

type diffResult struct {
	Source diffSource
	Refs   []lineRef
	Lines  []string
	Err    error
}

type diffSource string

const (
	sourceLocal  diffSource = "local"
	sourceBranch diffSource = "branch"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rms:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	return reviewTUI(args, loadDiffAsync(args, sourceLocal), stdout)
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
	source          diffSource
	sourceArgs      []string
	localAvailable  bool
	branchAvailable bool
	pr              *prContext
	prChecking      bool
	draft           reviewDraft
	selections      []displaySelection
	lines           []string
	cursor          int
	selectionAnchor *int
	top             int
	message         string
	loading         bool
	loadingFrame    int
}

func reviewTUI(args []string, initialDiff <-chan diffResult, stdout io.Writer) error {
	prResult := detectReviewContextAsync()
	state := &reviewState{
		args:       args,
		source:     sourceLocal,
		sourceArgs: args,
		prChecking: true,
		lines:      []string{loadingText(0)},
		loading:    true,
	}

	term, err := enterTerminal()
	if err != nil {
		result := <-initialDiff
		if result.Err != nil {
			return result.Err
		}
		fmt.Fprintln(stdout, strings.Join(result.Lines, "\n"))
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Interactive review needs a terminal. Run `go run .` directly, then use j/k to move, e to open, q to quit.")
		return nil
	}
	defer term.restore()

	keyResult := readKeyAsync(os.Stdin)
	loadingTick := time.NewTicker(60 * time.Millisecond)
	defer loadingTick.Stop()
	for {
		rows, cols := terminalSize()
		state.keepSelectionVisible(rows)
		render(stdout, state, rows, cols)

		var key string
		select {
		case result := <-initialDiff:
			applyInitialDiffResult(state, result, func() {
				loadingTick.Stop()
			})
			continue
		case context := <-prResult:
			state.applyReviewContext(context)
			continue
		case <-loadingTick.C:
			if state.loading {
				state.loadingFrame++
				state.lines = []string{loadingText(state.loadingFrame)}
			}
			continue
		case result := <-keyResult:
			if result.Err != nil {
				return result.Err
			}
			key = result.Key
			keyResult = nil
		}

		switch key {
		case keys.Q:
			return nil
		case keys.Esc:
			if state.selectionAnchor != nil {
				state.clearSelection()
				state.message = ""
				continue
			}
			return nil
		case "j", keys.Down:
			state.move(1)
		case "k", keys.Up:
			state.move(-1)
		case "h", keys.Left:
			state.selectSide("old")
		case "l", keys.Right:
			state.selectSide("new")
		case "g":
			state.moveTo(0)
		case "G":
			state.moveTo(len(state.selections) - 1)
		case keys.CtrlD, keys.PageDown:
			state.move(max(1, (rows-2)/2))
		case keys.CtrlU, keys.PageUp:
			state.move(-max(1, (rows-2)/2))
		case keys.Tab:
			if err := state.switchSource(); err != nil {
				state.message = err.Error()
			}
		case "e", keys.Enter:
			if !state.hasSelection() {
				state.message = "No changed line selected."
				continue
			}
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
			if err := state.reloadSource(); err != nil {
				state.message = err.Error()
				continue
			}
			state.message = "Reloaded diff."
		case "v":
			state.toggleSelection()
		case "R":
			state.startReview()
		case "c":
			if err := state.reviewComment(term); err != nil {
				state.message = err.Error()
			}
		case "s":
			if err := state.reviewSuggestion(term); err != nil {
				state.message = err.Error()
			}
		case "p":
			if err := state.submitReview(term); err != nil {
				state.message = err.Error()
			}
		case "D":
			state.discardReview()
		}
		if keyResult == nil {
			keyResult = readKeyAsync(os.Stdin)
		}
	}
}

func readKeyAsync(r io.Reader) <-chan keyResult {
	ch := make(chan keyResult, 1)
	go func() {
		key, err := keys.Read(r)
		ch <- keyResult{Key: key, Err: err}
	}()
	return ch
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
	s.loading = false
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

func applyInitialDiffResult(s *reviewState, result diffResult, stopLoading func()) {
	wasLoading := s.loading
	s.applyDiffResult(result)
	if wasLoading && !s.loading {
		stopLoading()
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

func loadingText(frame int) string {
	const text = "Gathering the diff"
	typed := min(len(text), 1+frame/2)
	cursor := " "
	if (frame/6)%2 == 0 {
		cursor = "\x1b[7m \x1b[0m"
	}
	return text[:typed] + cursor
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

func (s *reviewState) reviewComment(term *terminalState) error {
	return s.reviewWithBody(term, "")
}

func (s *reviewState) reviewSuggestion(term *terminalState) error {
	if !s.hasSelection() {
		return errors.New("No changed line selected.")
	}
	if err := s.requirePR("build suggestion"); err != nil {
		return err
	}
	reviewRange, err := s.currentRange()
	if err != nil {
		return err
	}
	if reviewRange.End.Side != "new" {
		return errors.New("Suggestions are only available on the right side.")
	}
	template, err := suggestionTemplate(reviewRange, s.pr)
	if err != nil {
		return err
	}
	return s.reviewWithBody(term, template)
}

func (s *reviewState) reviewWithBody(term *terminalState, template string) error {
	if !s.hasSelection() {
		return errors.New("No changed line selected.")
	}
	if err := s.requirePR("post review comments"); err != nil {
		return err
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
	if s.draft.Active {
		if err := github.AddPendingReviewComment(s.draft.ID, githubRange(reviewRange), body); err != nil {
			return err
		}
		s.draft.Count++
		s.clearSelection()
		if reviewRange.Start.Line == reviewRange.End.Line {
			s.message = fmt.Sprintf("Added draft comment %d on %s:%d.", s.draft.Count, reviewRange.End.File, reviewRange.End.Line)
		} else {
			s.message = fmt.Sprintf("Added draft comment %d on %s:%d-%d.", s.draft.Count, reviewRange.End.File, reviewRange.Start.Line, reviewRange.End.Line)
		}
		return nil
	}
	if err := github.PostReviewComment(s.pr, githubRange(reviewRange), body); err != nil {
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

func (s *reviewState) startReview() {
	if err := s.requirePR("start review"); err != nil {
		s.message = err.Error()
		return
	}
	if s.draft.Active {
		s.message = fmt.Sprintf("Review already active with %d draft %s.", s.draft.Count, plural(s.draft.Count, "comment", "comments"))
		return
	}
	reviewID, err := github.CreatePendingReview(s.pr)
	if err != nil {
		s.message = err.Error()
		return
	}
	s.draft = reviewDraft{Active: true, ID: reviewID}
	s.clearSelection()
	s.message = "Draft review started."
}

func (s *reviewState) discardReview() {
	if !s.draft.Active {
		s.message = "No draft review to delete."
		return
	}
	count := s.draft.Count
	if err := github.DeletePendingReview(s.draft.ID); err != nil {
		s.message = err.Error()
		return
	}
	s.draft = reviewDraft{}
	s.clearSelection()
	s.message = fmt.Sprintf("Deleted draft review with %d %s.", count, plural(count, "comment", "comments"))
}

func (s *reviewState) submitReview(term *terminalState) error {
	if err := s.requirePR("submit review"); err != nil {
		return err
	}
	if !s.draft.Active {
		return errors.New("No draft review active. Press R to start one.")
	}

	body, err := editReviewBody(term, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" && s.draft.Count == 0 {
		s.message = "Cancelled empty review."
		return nil
	}

	count := s.draft.Count
	if err := github.SubmitPendingReview(s.draft.ID, body); err != nil {
		return err
	}
	s.draft = reviewDraft{}
	s.clearSelection()
	s.message = fmt.Sprintf("Submitted review with %d %s.", count, plural(count, "comment", "comments"))
	return nil
}

func (s *reviewState) requirePR(action string) error {
	if s.pr != nil {
		return nil
	}
	if s.prChecking {
		return fmt.Errorf("Checking for an active GitHub PR. Cannot %s yet.", action)
	}
	return fmt.Errorf("No active GitHub PR found. Cannot %s.", action)
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
		return "", errors.New("No active GitHub PR found. Cannot build suggestion.")
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

func githubRange(reviewRange reviewRange) github.LineRange {
	return github.LineRange{
		Start: github.Line{
			File: reviewRange.Start.File,
			Line: reviewRange.Start.Line,
			Side: reviewRange.Start.Side,
		},
		End: github.Line{
			File: reviewRange.End.File,
			Line: reviewRange.End.Line,
			Side: reviewRange.End.Side,
		},
	}
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

func render(w io.Writer, state *reviewState, rows, cols int) {
	if rows < 8 {
		rows = 8
	}
	if cols < 40 {
		cols = 40
	}
	state.keepSelectionVisible(rows)
	bodyRows := rows - 2
	selectedLine := -1
	if state.hasSelection() {
		selectedLine = state.selectedDisplayLine()
	}

	fmt.Fprint(w, "\x1b[H\x1b[2J")
	for row := range bodyRows {
		lineIndex := state.top + row
		if lineIndex >= len(state.lines) {
			fmt.Fprint(w, "\x1b[K\r\n")
			continue
		}
		line := ansi.Truncate(state.lines[lineIndex], cols)
		if selection, ok := state.displayLineSelection(lineIndex, cols); ok {
			fmt.Fprintf(w, "%s\x1b[K\r\n", highlightSelectionSide(line, cols, selection))
		} else if state.hasSelection() && lineIndex == selectedLine {
			fmt.Fprintf(w, "%s\x1b[K\r\n", highlightSelectionSide(line, cols, state.selections[state.cursor]))
		} else {
			fmt.Fprintf(w, "%s\x1b[K\r\n", line)
		}
	}

	if state.message != "" {
		fmt.Fprintf(w, "%s\x1b[K\r\n", fit(" "+state.message, cols))
	} else {
		fmt.Fprint(w, "\x1b[K\r\n")
	}
	fmt.Fprintf(w, "\x1b[2m%s\x1b[0m\x1b[K", fit(helpText(state), cols))
}

func helpText(state *reviewState) string {
	nav := "h/j/k/l move  v select"
	if !state.hasSelection() {
		nav = "r reload"
	}
	sourceSwitch := ""
	if state.prChecking {
		sourceSwitch = "  checking PR"
	} else if state.source == sourceLocal && state.branchAvailable {
		sourceSwitch = "  Tab diff branch"
	} else if state.source == sourceBranch && state.localAvailable {
		sourceSwitch = "  Tab diff uncommitted"
	}
	if state.draft.Active {
		return fmt.Sprintf(" %s%s  c add comment  s add suggestion  p submit review  D delete draft  e open  r reload  q quit ", nav, sourceSwitch)
	}
	if state.prChecking {
		return fmt.Sprintf(" %s%s  e open  r reload  q quit ", nav, sourceSwitch)
	}
	if state.pr == nil {
		return fmt.Sprintf(" %s%s  e open  r reload  q quit ", nav, sourceSwitch)
	}
	return fmt.Sprintf(" %s%s  R start review  c comment  s suggest  e open  r reload  q quit ", nav, sourceSwitch)
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
	if s.selectionAnchor != nil {
		s.clearSelection()
		s.message = "Selection cleared."
		return
	}
	anchor := s.cursor
	s.selectionAnchor = &anchor
	s.message = "Selection started."
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

func fit(s string, width int) string {
	plainLen := len(ansi.Strip(s))
	if plainLen <= width {
		return s
	}
	return ansi.Truncate(s, width)
}

func highlightPlain(s string, width int) string {
	plain := ansi.Strip(s)
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
	plain := ansi.Strip(line)
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
			ansiEnd := ansi.End(s, i)
			if ansiEnd > i {
				if !inverse {
					out.WriteString(s[i:ansiEnd])
				}
				i = ansiEnd
				continue
			}
		}
		if !inverse && visible == start {
			out.WriteString("\x1b[0m\x1b[7m")
			inverse = true
		}
		if inverse && visible == end {
			out.WriteString("\x1b[0m")
			inverse = false
		}
		out.WriteByte(s[i])
		visible++
		i++
	}
	for visible < width {
		if !inverse && visible == start {
			out.WriteString("\x1b[0m\x1b[7m")
			inverse = true
		}
		if inverse && visible == end {
			out.WriteString("\x1b[0m")
			inverse = false
		}
		out.WriteByte(' ')
		visible++
	}
	if inverse {
		out.WriteString("\x1b[0m")
	}
	out.WriteString("\x1b[0m")
	return out.String()
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
