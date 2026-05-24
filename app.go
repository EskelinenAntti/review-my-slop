package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/keys"
)

func run(args []string, stdout io.Writer) error {
	return reviewTUI(args, loadDiffAsync(args, sourceLocal), stdout)
}

func reviewTUI(args []string, initialDiff <-chan diffResult, stdout io.Writer) error {
	prResult := detectReviewContextAsync()
	state := newReviewState(args)

	term, err := enterTerminal()
	if err != nil {
		result := <-initialDiff
		if result.Err != nil {
			return result.Err
		}
		fmt.Fprintln(stdout, strings.Join(result.Lines, "\n"))
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Interactive review needs a terminal. Run `slop` directly, then use j/k to move, e to open, q to quit.")
		return nil
	}
	defer term.restore()

	keyResult := readKeyAsync(os.Stdin)
	for {
		rows, cols := terminalSize()
		state.keepSelectionVisible(rows)
		render(stdout, state, rows, cols)

		var key string
		select {
		case result := <-initialDiff:
			state.applyDiffResult(result)
			continue
		case context := <-prResult:
			state.applyReviewContext(context)
			continue
		case result := <-keyResult:
			if result.Err != nil {
				return result.Err
			}
			key = result.Key
			keyResult = nil
		}

		if quit := state.handleKey(key, term, rows); quit {
			return nil
		}
		if keyResult == nil {
			keyResult = readKeyAsync(os.Stdin)
		}
	}
}

func newReviewState(args []string) *reviewState {
	return &reviewState{
		args:       args,
		source:     sourceLocal,
		sourceArgs: args,
		prChecking: true,
		lines:      []string{"Gathering the diff..."},
	}
}

func (s *reviewState) handleKey(key string, term *terminalState, rows int) bool {
	switch key {
	case keys.Q:
		return true
	case keys.Esc:
		if s.selectionAnchor != nil {
			s.clearSelection()
			s.message = ""
			return false
		}
		return true
	case "j", keys.Down:
		s.move(1)
	case "k", keys.Up:
		s.move(-1)
	case "h", keys.Left:
		s.selectSide("old")
	case "l", keys.Right:
		s.selectSide("new")
	case "g":
		s.moveTo(0)
	case "G":
		s.moveTo(len(s.changedLines) - 1)
	case keys.CtrlD, keys.PageDown:
		s.move(max(1, (rows-2)/2))
	case keys.CtrlU, keys.PageUp:
		s.move(-max(1, (rows-2)/2))
	case "e", keys.Enter:
		s.openSelectedLine(term)
	case "o":
		s.openPR(term)
	case "r":
		if err := s.reloadSource(); err != nil {
			s.message = err.Error()
			return false
		}
		s.message = "Reloaded diff."
	case "v":
		s.toggleSelection()
	case "R":
		s.startReview()
	case "c":
		if err := s.reviewComment(term); err != nil {
			s.message = err.Error()
		}
	case "s":
		if err := s.reviewSuggestion(term); err != nil {
			s.message = err.Error()
		}
	case "p":
		if err := s.submitReview(term); err != nil {
			s.message = err.Error()
		}
	case "D":
		s.discardReview()
	}
	return false
}

func (s *reviewState) openSelectedLine(term *terminalState) {
	if !s.hasChangedLines() {
		s.message = "No changed line selected."
		return
	}
	s.clearSelection()
	ref := s.current()
	if ref.Side == "old" {
		s.message = fmt.Sprintf("Cannot open deleted line %s:%d", ref.File, ref.Line)
		return
	}
	if err := withNormalTerminal(term, func() error {
		return openEditor(ref.File, ref.Line)
	}); err != nil {
		s.message = err.Error()
		return
	}
	if s.source == sourceLocal {
		if err := s.reloadSourceAt(ref); err != nil {
			s.message = err.Error()
			return
		}
	}
	s.message = fmt.Sprintf("Opened %s:%d", ref.File, ref.Line)
}

func readKeyAsync(r io.Reader) <-chan keyResult {
	ch := make(chan keyResult, 1)
	go func() {
		key, err := keys.Read(r)
		ch <- keyResult{Key: key, Err: err}
	}()
	return ch
}
