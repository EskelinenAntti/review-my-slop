package keys

import (
	"io"
)

const (
	Q        = "q"
	Tab      = "tab"
	Esc      = "esc"
	Enter    = "enter"
	Up       = "up"
	Down     = "down"
	Left     = "left"
	Right    = "right"
	PageUp   = "pageup"
	PageDown = "pagedown"
	CtrlD    = "ctrl-d"
	CtrlU    = "ctrl-u"
)

// Read decodes one terminal keypress into the command names used by the TUI.
func Read(r io.Reader) (string, error) {
	var buf [1]byte
	if _, err := r.Read(buf[:]); err != nil {
		return "", err
	}
	switch buf[0] {
	case 3:
		return Q, nil
	case '\t':
		return Tab, nil
	case 4:
		return CtrlD, nil
	case 21:
		return CtrlU, nil
	case '\r', '\n':
		return Enter, nil
	case 27:
		return readEscape(r)
	default:
		return string(buf[0]), nil
	}
}

func readEscape(r io.Reader) (string, error) {
	var seq [2]byte
	if _, err := io.ReadFull(r, seq[:1]); err != nil {
		return Esc, nil
	}
	if seq[0] != '[' {
		return Esc, nil
	}
	if _, err := io.ReadFull(r, seq[1:]); err != nil {
		return Esc, nil
	}
	switch seq[1] {
	case 'A':
		return Up, nil
	case 'B':
		return Down, nil
	case 'C':
		return Right, nil
	case 'D':
		return Left, nil
	case '5':
		discardTerminator(r)
		return PageUp, nil
	case '6':
		discardTerminator(r)
		return PageDown, nil
	default:
		return Esc, nil
	}
}

func discardTerminator(r io.Reader) {
	var discard [1]byte
	_, _ = io.ReadFull(r, discard[:])
}
