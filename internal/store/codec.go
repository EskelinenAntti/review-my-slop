package store

import (
	"encoding/json"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
)

func encodeComment(item comment.Comment) ([]byte, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("encode comment: %w", err)
	}
	return data, nil
}

func decodeComment(data []byte) (comment.Comment, error) {
	var item comment.Comment
	if err := json.Unmarshal(data, &item); err != nil {
		return comment.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return item, nil
}
