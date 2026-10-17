package editor

import (
	"context"
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := func() (string, error) {
		file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
		if err != nil {
			return "", fmt.Errorf("create comment file: %w", err)
		}
		path := file.Name()
		if _, err := file.WriteString(func() string {
			var body string = "existing comment"
			if len(anchor.QuotedLines) == 0 {
				return body
			}
			lines := func() []string {
				lines := make([]string, 0, len(anchor.QuotedLines))
				for _, line := range anchor.QuotedLines {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			}()
			var draft strings.Builder
			_, _ = draft.WriteString(body)
			if body != "" && !strings.HasSuffix(body, "\n") {
				_ = draft.WriteByte('\n')
			}
			_ = draft.WriteByte('\n')
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			_, _ = draft.WriteString(fence)
			_, _ = draft.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = draft.WriteString(line)
				_ = draft.WriteByte('\n')
			}
			_, _ = draft.WriteString(fence)
			_ = draft.WriteByte('\n')
			return draft.String()
		}()); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", fmt.Errorf("write comment file: %w", err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("close comment file: %w", err)
		}
		return path, nil
	}()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "existing comment\n\n```suggestion\nnew()\n```\n"
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 ||
		string(body) != want {
		t.Fatalf("path=%q mode=%o body=%q", path, info.Mode().Perm(), body)
	}
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := comments.Anchor{
			QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"},
		}
		draft := func() string {
			var body string = "explain this"
			if len(anchor.QuotedLines) == 0 {
				return body
			}
			lines := func() []string {
				lines := make([]string, 0, len(anchor.QuotedLines))
				for _, line := range anchor.QuotedLines {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			}()
			var draft strings.Builder
			_, _ = draft.WriteString(body)
			if body != "" && !strings.HasSuffix(body, "\n") {
				_ = draft.WriteByte('\n')
			}
			_ = draft.WriteByte('\n')
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			_, _ = draft.WriteString(fence)
			_, _ = draft.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = draft.WriteString(line)
				_ = draft.WriteByte('\n')
			}
			_, _ = draft.WriteString(fence)
			_ = draft.WriteByte('\n')
			return draft.String()
		}()
		if !strings.Contains(
			draft,
			"`````suggestion",
		) || func() string {
			if len(anchor.QuotedLines) == 0 {
				return draft
			}
			lines := func() []string {
				lines := make([]string, 0, len(anchor.QuotedLines))
				for _, line := range anchor.QuotedLines {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			}()
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			var suggestion strings.Builder
			_, _ = suggestion.WriteString(fence)
			_, _ = suggestion.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = suggestion.WriteString(line)
				_ = suggestion.WriteByte('\n')
			}
			_, _ = suggestion.WriteString(fence)
			start := strings.Index(draft, suggestion.String())
			if start < 0 {
				return draft
			}
			end := start + suggestion.Len()
			before := strings.TrimRight(draft[:start], "\n")
			after := draft[end:]
			if strings.TrimSpace(after) == "" {
				return before
			}
			return before + "\n" + strings.TrimLeft(after, "\n")
		}() != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := comments.Anchor{
			QuotedLines: []string{" unchanged()", "-old()", "+new()"},
		}
		want := "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"
		if got := func() string {
			var body string = "comment"
			if len(anchor.QuotedLines) == 0 {
				return body
			}
			lines := func() []string {
				lines := make([]string, 0, len(anchor.QuotedLines))
				for _, line := range anchor.QuotedLines {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			}()
			var draft strings.Builder
			_, _ = draft.WriteString(body)
			if body != "" && !strings.HasSuffix(body, "\n") {
				_ = draft.WriteByte('\n')
			}
			_ = draft.WriteByte('\n')
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			_, _ = draft.WriteString(fence)
			_, _ = draft.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = draft.WriteString(line)
				_ = draft.WriteByte('\n')
			}
			_, _ = draft.WriteString(fence)
			_ = draft.WriteByte('\n')
			return draft.String()
		}(); got != want {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		quoted := []string{"-old()", "+new()"}
		if got := func() string {
			if len(quoted) == 0 {
				return body
			}
			lines := func() []string {
				lines := make([]string, 0, len(quoted))
				for _, line := range quoted {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			}()
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			var suggestion strings.Builder
			_, _ = suggestion.WriteString(fence)
			_, _ = suggestion.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = suggestion.WriteString(line)
				_ = suggestion.WriteByte('\n')
			}
			_, _ = suggestion.WriteString(fence)
			start := strings.Index(body, suggestion.String())
			if start < 0 {
				return body
			}
			end := start + suggestion.Len()
			before := strings.TrimRight(body[:start], "\n")
			after := body[end:]
			if strings.TrimSpace(after) == "" {
				return before
			}
			return before + "\n" + strings.TrimLeft(after, "\n")
		}(); got != body {
			t.Fatalf("body = %q", got)
		}
	})
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	command := exec.CommandContext(context.Background(), "sh", "-c", "printf 'edited externally' >"+" "+(func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	})(path))
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	body, err := func() (string, error) {
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read comment file: %w", err)
		}
		return func() string {
			if len(comments.Anchor{}.QuotedLines) == 0 {
				return string(body)
			}
			lines := (func(quoted []string) []string {
				lines := make([]string, 0, len(quoted))
				for _, line := range quoted {
					if line != "" && line[0] != '-' {
						lines = append(lines, line[1:])
					}
				}
				return lines
			})(comments.Anchor{}.QuotedLines)
			fence := (func(lines []string) string {
				longest := 0
				for _, line := range lines {
					run := 0
					for _, char := range line {
						if char == '`' {
							run++
							longest = max(longest, run)
						} else {
							run = 0
						}
					}
				}
				return strings.Repeat("`", max(3, longest+1))
			})(lines)
			var suggestion strings.Builder
			_, _ = suggestion.WriteString(fence)
			_, _ = suggestion.WriteString("suggestion\n")
			for _, line := range lines {
				_, _ = suggestion.WriteString(line)
				_ = suggestion.WriteByte('\n')
			}
			_, _ = suggestion.WriteString(fence)
			start := strings.Index(string(body), suggestion.String())
			if start < 0 {
				return string(body)
			}
			end := start + suggestion.Len()
			before := strings.TrimRight(string(body)[:start], "\n")
			after := string(body)[end:]
			if strings.TrimSpace(after) == "" {
				return before
			}
			return before + "\n" + strings.TrimLeft(after, "\n")
		}(), nil
	}()
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := func() string {
		var body string = "body"
		if len(anchor.QuotedLines) == 0 {
			return body
		}
		lines := (func(quoted []string) []string {
			lines := make([]string, 0, len(quoted))
			for _, line := range quoted {
				if line != "" && line[0] != '-' {
					lines = append(lines, line[1:])
				}
			}
			return lines
		})(anchor.QuotedLines)
		var draft strings.Builder
		_, _ = draft.WriteString(body)
		if body != "" && !strings.HasSuffix(body, "\n") {
			_ = draft.WriteByte('\n')
		}
		_ = draft.WriteByte('\n')
		fence := (func(lines []string) string {
			longest := 0
			for _, line := range lines {
				run := 0
				for _, char := range line {
					if char == '`' {
						run++
						longest = max(longest, run)
					} else {
						run = 0
					}
				}
			}
			return strings.Repeat("`", max(3, longest+1))
		})(lines)
		_, _ = draft.WriteString(fence)
		_, _ = draft.WriteString("suggestion\n")
		for _, line := range lines {
			_, _ = draft.WriteString(line)
			_ = draft.WriteByte('\n')
		}
		_, _ = draft.WriteString(fence)
		_ = draft.WriteByte('\n')
		return draft.String()
	}()
	got := func() string {
		if len(anchor.QuotedLines) == 0 {
			return draft
		}
		lines := (func(quoted []string) []string {
			lines := make([]string, 0, len(quoted))
			for _, line := range quoted {
				if line != "" && line[0] != '-' {
					lines = append(lines, line[1:])
				}
			}
			return lines
		})(anchor.QuotedLines)
		fence := (func(lines []string) string {
			longest := 0
			for _, line := range lines {
				run := 0
				for _, char := range line {
					if char == '`' {
						run++
						longest = max(longest, run)
					} else {
						run = 0
					}
				}
			}
			return strings.Repeat("`", max(3, longest+1))
		})(lines)
		var suggestion strings.Builder
		_, _ = suggestion.WriteString(fence)
		_, _ = suggestion.WriteString("suggestion\n")
		for _, line := range lines {
			_, _ = suggestion.WriteString(line)
			_ = suggestion.WriteByte('\n')
		}
		_, _ = suggestion.WriteString(fence)
		start := strings.Index(draft, suggestion.String())
		if start < 0 {
			return draft
		}
		end := start + suggestion.Len()
		before := strings.TrimRight(draft[:start], "\n")
		after := draft[end:]
		if strings.TrimSpace(after) == "" {
			return before
		}
		return before + "\n" + strings.TrimLeft(after, "\n")
	}()
	if got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
