package patch

// LineNumber is one-based; zero means absent on that side.
type LineNumber int

// LineKind identifies context, added or removed diff lines.
type LineKind uint8

// Context starts the diff line kinds, followed by additions and deletions.
const (
	Context LineKind = iota
	Addition
	Deletion
)
