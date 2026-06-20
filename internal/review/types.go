package review

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
