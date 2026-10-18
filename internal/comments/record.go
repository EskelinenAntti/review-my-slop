package comments

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func validateComment(comment Comment) error {
	if len(comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func validateCommentIdentity(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return nil
}

func encodeComment(comment Comment) ([]byte, error) {
	if err := validateComment(comment); err != nil {
		return nil, err
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

type commentRecord struct {
	ID            string         `json:"id"`
	Repository    string         `json:"repository"`
	CreatedAt     time.Time      `json:"created_at"`
	Anchor        Anchor         `json:"anchor"`
	Body          string         `json:"body"`
	LegacyComment *legacyComment `json:"comment"`
}

func decodeComment(data []byte) (Comment, error) {
	var record commentRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	comment := Comment{
		ID:         record.ID,
		Repository: record.Repository,
		CreatedAt:  record.CreatedAt,
		Anchor:     record.Anchor,
		Body:       record.Body,
	}
	if record.LegacyComment != nil {
		comment.Anchor = record.LegacyComment.Anchor
		comment.Body = record.LegacyComment.Body
	}
	return comment, nil
}
