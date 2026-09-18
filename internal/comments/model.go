package comments

import "time"

// Anchor identifies source lines without referring to terminal rows.
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

// Snapshot is the immutable delivery view read before a prompt is written.
// Acknowledge removes only records whose serialized value is still identical
// to the snapshot, so an edit or concurrent replacement remains pending.
type Snapshot struct {
	repository string
	items      []snapshotItem
}

type snapshotItem struct {
	comment Comment
	encoded []byte
}

func (s Snapshot) Repository() string { return s.repository }

func (s Snapshot) Comments() []Comment {
	comments := make([]Comment, 0, len(s.items))
	for _, item := range s.items {
		comments = append(comments, cloneComment(item.comment))
	}
	return comments
}

func cloneComment(comment Comment) Comment {
	comment.Anchor.QuotedLines = append([]string(nil), comment.Anchor.QuotedLines...)
	return comment
}
