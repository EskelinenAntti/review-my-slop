package patch

// Kind identifies the comparison used to load a patch.
type Kind uint8

// Patch comparisons include working-tree changes and branch changes.
const (
	Unstaged Kind = iota

	Branch
)

// Patch contains the changes for a repository and comparison.
type Patch struct {
	Root   string
	Kind   Kind
	Branch string
	Files  []File
}

// File contains a changed file, its source text, metadata, and hunks.
type File struct {
	OldPath     string
	NewPath     string
	DisplayPath string
	OldSource   string
	NewSource   string
	Metadata    []string
	Hunks       []Hunk
}

// Hunk groups adjacent changed and context lines.
type Hunk struct {
	Header string
	Lines  []Line
}
