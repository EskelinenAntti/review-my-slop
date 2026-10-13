package comments

import "time"

func testComment(repository, body string) Comment {
	return Comment{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(1, 0).UTC(),
		Anchor:     Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1},
		Body:       body,
	}
}
