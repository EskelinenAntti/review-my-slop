package ui

type command uint8

const (
	commandNone command = iota
	commandQuit
	commandHelp
	commandSearch
	commandRepeatSearchForward
	commandRepeatSearchBackward
	commandMoveForward
	commandMoveBackward
	commandScrollLeft
	commandScrollRight
	commandScrollStart
	commandScrollEnd
	commandHalfPageForward
	commandHalfPageBackward
	commandFirstLine
	commandLastLine
	commandAlignTop
	commandAlignMiddle
	commandAlignBottom
	commandJumpFileForward
	commandJumpFileBackward
	commandSwitchLeft
	commandSwitchRight
	commandSwitchOther
	commandToggleSelection
	commandCancelSelection
	commandComment
	commandOpenSource
	commandShowComments
	commandRefresh
	commandToggleBranch
	commandToggleLayout
)

type keyPrefix uint8

const (
	prefixNone keyPrefix = iota
	prefixGo
	prefixAlign
	prefixFileForward
	prefixFileBackward
	prefixPane
)

type keyDecoder struct{ pending keyPrefix }

func (d *keyDecoder) Decode(name string) command {
	pending := d.pending
	d.pending = prefixNone
	switch pending {
	case prefixFileForward:
		if name == "f" {
			return commandJumpFileForward
		}
		return commandNone
	case prefixFileBackward:
		if name == "f" {
			return commandJumpFileBackward
		}
		return commandNone
	case prefixAlign:
		switch name {
		case "z":
			return commandAlignMiddle
		case "t":
			return commandAlignTop
		case "b":
			return commandAlignBottom
		}
		return commandNone
	case prefixPane:
		switch name {
		case "h":
			return commandSwitchLeft
		case "l":
			return commandSwitchRight
		case "ctrl+w":
			return commandSwitchOther
		}
		return commandNone
	case prefixGo:
		if name == "g" {
			return commandFirstLine
		}
	}

	switch name {
	case "ctrl+c", "q":
		return commandQuit
	case "?":
		return commandHelp
	case "/":
		return commandSearch
	case "n":
		return commandRepeatSearchForward
	case "N":
		return commandRepeatSearchBackward
	case "j", "down":
		return commandMoveForward
	case "k", "up":
		return commandMoveBackward
	case "h", "left":
		return commandScrollLeft
	case "l", "right":
		return commandScrollRight
	case "0":
		return commandScrollStart
	case "$":
		return commandScrollEnd
	case "ctrl+d":
		return commandHalfPageForward
	case "ctrl+u":
		return commandHalfPageBackward
	case "ctrl+w":
		d.pending = prefixPane
	case "g":
		d.pending = prefixGo
	case "G":
		return commandLastLine
	case "z":
		d.pending = prefixAlign
	case "]":
		d.pending = prefixFileForward
	case "[":
		d.pending = prefixFileBackward
	case "v":
		return commandToggleSelection
	case "esc":
		return commandCancelSelection
	case "c":
		return commandComment
	case "e":
		return commandOpenSource
	case "C":
		return commandShowComments
	case "R":
		return commandRefresh
	case "tab":
		return commandToggleBranch
	case "t":
		return commandToggleLayout
	}
	return commandNone
}

type keyBinding struct{ keys, description string }

var browseBindings = []keyBinding{
	{"j/k, arrows", "move"},
	{"h/l, left/right", "scroll horizontally"},
	{"Ctrl-w h/l/w", "switch side-by-side pane"},
	{"0/$", "start/end of lines"},
	{"gg/G", "first/last changed line"},
	{"zz/zt/zb", "center/top/bottom current line"},
	{"Ctrl-d/Ctrl-u", "half-page down/up"},
	{"/", "search patch text"},
	{"n/N", "next/previous search match"},
	{"]f/[f", "next/previous file"},
	{"v", "select a line range"},
	{"c", "comment on selection/current line"},
	{"e", "open current line in $EDITOR"},
	{"C", "view comments"},
	{"R", "refresh patch"},
	{"Tab", "toggle local/branch changes"},
	{"t", "toggle unified/side-by-side"},
	{"q", "quit"},
}
