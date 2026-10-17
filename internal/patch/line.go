package patch

// Line contains display-safe text and its original and updated line numbers.
type Line struct {
	Kind      LineKind
	Text      string
	OldNumber LineNumber
	NewNumber LineNumber
}

// LineNumber is a one-based source line number; zero means no line.
type LineNumber int

// LineKind distinguishes context, added, and deleted lines.
type LineKind uint8

// Line kinds describe each side of a change.
const (
	Context LineKind = iota
	Addition
	Deletion
)
