package tui

import (
	"io"
)

type Key string

const (
	KeyUnknown  Key = ""
	KeyCtrlC    Key = "ctrl-c"
	KeyCtrlD    Key = "ctrl-d"
	KeyCtrlN    Key = "ctrl-n"
	KeyCtrlP    Key = "ctrl-p"
	KeyCtrlU    Key = "ctrl-u"
	KeyEscape   Key = "escape"
	KeyUp       Key = "up"
	KeyDown     Key = "down"
	KeyLeft     Key = "left"
	KeyRight    Key = "right"
	KeyPageUp   Key = "page-up"
	KeyPageDown Key = "page-down"
)

func ReadKey(r io.Reader) (Key, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return KeyUnknown, err
	}

	switch b[0] {
	case 3:
		return KeyCtrlC, nil
	case 4:
		return KeyCtrlD, nil
	case 14:
		return KeyCtrlN, nil
	case 16:
		return KeyCtrlP, nil
	case 21:
		return KeyCtrlU, nil
	case 27:
		return readEscape(r)
	default:
		return Key(string(b[0])), nil
	}
}

func readEscape(r io.Reader) (Key, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return KeyEscape, nil
	}
	if b[0] != '[' {
		return KeyEscape, nil
	}
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return KeyEscape, nil
	}
	switch b[0] {
	case 'A':
		return KeyUp, nil
	case 'B':
		return KeyDown, nil
	case 'C':
		return KeyRight, nil
	case 'D':
		return KeyLeft, nil
	case '5':
		discardByte(r)
		return KeyPageUp, nil
	case '6':
		discardByte(r)
		return KeyPageDown, nil
	default:
		return KeyEscape, nil
	}
}

func discardByte(r io.Reader) {
	var b [1]byte
	_, _ = io.ReadFull(r, b[:])
}
