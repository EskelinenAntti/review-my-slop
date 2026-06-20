package review

import "time"

type Anchor struct {
	FilePath    string   `json:"file"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type Comment struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"`
	CreatedAt  time.Time `json:"created_at"`
	Anchor     Anchor    `json:"anchor"`
	Body       string    `json:"body"`
}
