package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/ansi"
)

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
	suppressedStyle := ""

	for i := 0; i < len(s) && visible < width; {
		if s[i] == '\x1b' {
			ansiEnd := ansi.End(s, i)
			if ansiEnd > i {
				if !inverse {
					out.WriteString(s[i:ansiEnd])
				} else if s[i:ansiEnd] == "\x1b[0m" {
					suppressedStyle = ""
				} else {
					suppressedStyle = s[i:ansiEnd]
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
			if suppressedStyle != "" {
				out.WriteString(suppressedStyle)
				suppressedStyle = ""
			}
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
			if suppressedStyle != "" {
				out.WriteString(suppressedStyle)
				suppressedStyle = ""
			}
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
