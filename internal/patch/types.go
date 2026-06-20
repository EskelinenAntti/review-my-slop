package patch

type Patch struct {
	Repository  string
	Fingerprint string
	Files       []File
}

type File struct {
	OldPath     string
	NewPath     string
	DisplayPath string
	OldSource   string
	NewSource   string
	Metadata    []string
	Hunks       []Hunk
}

func (f File) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
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

type LineNumber int

type LineKind uint8

const (
	Context LineKind = iota
	Addition
	Deletion
)
