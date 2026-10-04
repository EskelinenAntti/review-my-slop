package comments

import (
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Anchor = patch.Anchor

type Comment struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"`
	CreatedAt  time.Time `json:"created_at"`
	Anchor     Anchor    `json:"anchor"`
	Body       string    `json:"body"`
}
