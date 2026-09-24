// Package patch represents and loads the repository change set being reviewed.
// Repository and Git details stay behind this package's loading API.
package patch

type (
	Patch struct {
		Repository, Fingerprint string
		Files                   []File
	}
	File struct {
		OldPath, NewPath, DisplayPath, OldSource, NewSource string
		Metadata                                            []string
		Hunks                                               []Hunk
	}
	Hunk struct {
		Header string
		Lines  []Line
	}
	Line struct {
		Kind                 LineKind
		Text                 string
		OldNumber, NewNumber LineNumber
	}
	LineNumber int
	LineKind   uint8
)

const (
	Context LineKind = iota
	Addition
	Deletion
)
