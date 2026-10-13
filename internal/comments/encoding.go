package comments

import (
	"encoding/json"
	"fmt"
	"time"
)

type legacyComment struct {
	Anchor Anchor `json:"anchor"`
	Body   string `json:"body"`
}

type legacyRecord struct {
	ID         string         `json:"id"`
	Repository string         `json:"repository"`
	CreatedAt  time.Time      `json:"created_at"`
	Comment    *legacyComment `json:"comment"`
}

func decodeCommentRecord(data []byte) (Comment, error) {
	var legacy legacyRecord
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return Comment{
			ID:         legacy.ID,
			Repository: legacy.Repository,
			CreatedAt:  legacy.CreatedAt,
			Anchor:     legacy.Comment.Anchor,
			Body:       legacy.Comment.Body,
		}, nil
	}
	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}
