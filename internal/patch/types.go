package patch

// Kind selects which changes to review.
type Kind uint8

const (
	// Unstaged compares the index to the working tree and includes untracked files.
	Unstaged Kind = iota
	// Branch compares the default branch's merge base to the working tree and includes untracked files.
	Branch
)

// Patch is a fully loaded snapshot. Its data belongs to the caller and is
// treated as read-only; exported fields do not enforce immutability.
type Patch struct {
	Root   string
	Kind   Kind
	Branch string // Discovered default branch, or empty if unavailable.
	Files  []File
}

// File holds paths, source text, metadata, and hunks for a changed file.
type File struct {
	// Paths are repository-relative; empty means absent on that side.
	OldPath     string
	NewPath     string
	DisplayPath string
	OldSource   string
	NewSource   string
	Metadata    []string
	Hunks       []Hunk
}

// Hunk groups diff lines under a unified diff header.
type Hunk struct {
	Header string
	Lines  []Line
}

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
