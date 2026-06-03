package diffparse

type Location struct {
	File string
	Line int
}

type Line struct {
	Text       string
	Location   Location
	Header     bool
	Selectable bool
	Editable   bool
}

type Parser interface {
	Parse(lines []string) []Line
}
