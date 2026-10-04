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

type Hunk struct {
	Header string
	Lines  []Line
}

type Line struct {
	Kind      LineKind
	Text      string
	OldNumber LineNumber
	NewNumber LineNumber
}

// LineNumber is one-based; zero means absent on that side.
type LineNumber int

type LineKind uint8

const (
	Context LineKind = iota
	Addition
	Deletion
)
