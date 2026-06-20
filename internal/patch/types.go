package patch

type Patch struct {
	Repository  string
	Fingerprint string
	Base        string
	Files       []File
}

type File struct {
	OldName   string
	NewName   string
	Name      string
	Language  string
	OldSource string
	NewSource string
	Metadata  []string
	Binary    bool
	Hunks     []Hunk
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
