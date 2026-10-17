package comments

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func encodeComment(comment Comment) ([]byte, error) {
	if len(comment.Body) == 0 {
		return nil, errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return nil, fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("encode comment: %w", err)
	}
	return data, nil
}

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

func decodeComment(data []byte) (Comment, error) {
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
