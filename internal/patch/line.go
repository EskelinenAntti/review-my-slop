package patch

// Line holds diff text and its old and new source line numbers.
type Line struct {
	Kind      LineKind
	Text      string
	OldNumber LineNumber
	NewNumber LineNumber
}

// LineNumber is one-based; zero means absent on that side.
type LineNumber int

// LineKind distinguishes unchanged, added, and deleted diff lines.
type LineKind uint8

// Context, Addition, and Deletion identify the kind of a diff line.
const (
	Context LineKind = iota
	Addition
	Deletion
)

// DiffPrefix returns the marker used to represent this kind in diff text.
func (kind LineKind) DiffPrefix() string {
	switch kind {
	case Addition:
		return "+"
	case Deletion:
		return "-"
	default:
		return " "
	}
}
