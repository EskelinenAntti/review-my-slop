package review

type LineKind uint8

const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

type Line struct {
	Kind      LineKind
	Text      string
	OldNumber int
	NewNumber int
}

type Hunk struct {
	Header string
	Lines  []Line
}

type File struct {
	OldPath   string
	NewPath   string
	Display   string
	Language  string
	OldSource string
	NewSource string
	Hunks     []Hunk
	Metadata  []string
	Binary    bool
}

type Diff struct {
	Repository  string
	Fingerprint string
	Base        string
	Files       []File
}

type Anchor struct {
	File        string   `json:"file"`
	Hunk        string   `json:"hunk"`
	StartRow    int      `json:"start_row"`
	EndRow      int      `json:"end_row"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type Comment struct {
	Anchor Anchor `json:"anchor"`
	Body   string `json:"body"`
}

type StoredComment struct {
	ID      string
	Comment Comment
}
