// Package diff contains the domain model for a reviewable set of changes.
//
// Values in this package are deliberately independent of terminal layout. Paths
// and source text are kept in their repository form; presentation code is
// responsible for escaping them before writing to a terminal.
package diff

// ChangeSet is the complete set of changes currently being reviewed.
type ChangeSet struct {
	Repository  string
	Fingerprint string
	Files       []File
}

// File is one changed repository path. OldPath and NewPath are raw repository
// paths. An empty side represents /dev/null in the Git diff.
type File struct {
	OldPath   string
	NewPath   string
	OldSource string
	NewSource string
	Metadata  []string
	Hunks     []Hunk
}

// Path returns the path that identifies the file in the working tree when one
// exists, falling back to the old path for deletions.
func (f File) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

// Key is the semantic identity of a file across presentation rebuilds.
type FileKey struct {
	OldPath string
	NewPath string
}

// Key returns the semantic identity of f. It is not a screen-row index.
func (f File) Key() FileKey {
	return FileKey{OldPath: f.OldPath, NewPath: f.NewPath}
}

// SameFile reports whether two file values refer to the same change. Git can
// omit one side for additions and deletions, so either shared non-empty path is
// sufficient.
func SameFile(first, second File) bool {
	keysMatch := first.Key() == second.Key() && (first.OldPath != "" || first.NewPath != "")
	return keysMatch ||
		(first.OldPath != "" && first.OldPath == second.OldPath) ||
		(first.NewPath != "" && first.NewPath == second.NewPath)
}

// Hunk is a contiguous region of changed lines.
type Hunk struct {
	Header string
	Lines  []Line
}

// Line is one semantic diff line. A zero line number means that the line does
// not exist on that side of the change.
type Line struct {
	Kind      LineKind
	Text      string
	OldNumber LineNumber
	NewNumber LineNumber
}

// LineNumber is a source-file line number, not a visual row.
type LineNumber int

// LineKind describes which versions contain a line.
type LineKind uint8

const (
	Context LineKind = iota
	Addition
	Deletion
)

// Changed reports whether the line exists on only one side of the change.
func (line Line) Changed() bool {
	return line.Kind == Addition || line.Kind == Deletion
}

// Number returns the source line number for the requested side.
func (line Line) Number(oldSide bool) LineNumber {
	if oldSide {
		return line.OldNumber
	}
	return line.NewNumber
}
