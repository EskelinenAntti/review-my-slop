package

// Package frame provides the shared screen composition used by UI screens.
main

import (
	bytes "bytes"
	cmp "cmp"
	context "context"
	json "encoding/json"
	errors "errors"
	fmt "fmt"
	io "io"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	regexp "regexp"
	slices "slices"
	strconv "strconv"
	strings "strings"
	time "time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
	term "github.com/charmbracelet/x/term"
	diff "github.com/sourcegraph/go-diff/diff"
	bolt "go.etcd.io/bbolt"
)

// Render composes a header, padded body, footer, and final blank line.

// replaceView translates interactive state before replacing the row projection.
// Source identity is shared by focus, selection endpoints, and the search
// origin.

// WritePending writes the repository's pending feedback and acknowledges only
// those comments after successful output. Failed output leaves them pending.

// WritePrompt writes comments as Markdown feedback for a coding agent.

// Store persists pending review comments in a database at Path.

// DefaultPath returns the database path in the application data directory.

// OpenDefault returns a Store configured with the default database path.

type diffscreenCursorIdentity struct {
	file   patchFile
	hunk   patchHunk
	line   patchLine
	cursor diffscreenDiffCursor
	valid  bool
}

func (m appModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(func() string {
		switch m.mode {
		case appModeHelp:
			bindings := []helpscreenBinding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return (func(bindings []helpscreenBinding, width, height int) string {
				column := 0
				for _, binding := range bindings {
					column = max(column, lipgloss.Width(binding.Keys))
				}
				body := []string{""}
				for _, binding := range bindings {
					line := binding.Keys + strings.Repeat(
						" ",
						column-lipgloss.Width(binding.Keys),
					) + "  " + binding.Description
					body = append(body, ansi.Truncate(line, max(20, width), ""))
				}
				return (func(header string, body []string, footer string, height int) string {
					bodyHeight := (func(height int) int { return max(1, height-3) })(height)
					lines := make([]string,

						// Add validates and stores a comment, assigning an ID and timestamp when
						// absent.
						0, bodyHeight+3)
					lines = append(lines, header)
					lines = append(lines, body[:min(len(body), bodyHeight)]...)
					for len(lines) < bodyHeight+1 {
						lines = append(lines, "")
					}
					lines = append(lines, footer, "")
					return strings.Join(lines, "\n")
				})(
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan).Render(
						"review-my-slop help",
					),
					body,
					lipgloss.NewStyle().Faint(true).Render("? or Esc closes help"),
					height,
				)
			})(bindings, m.width, m.height)
		case appModeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}())
	result.AltScreen = true
	result.ReportFocus = true
	result.MouseMode = tea.MouseModeCellMotion
	return result
}

func (s commentsStore) WritePending(w io.Writer, repository string) error {
	pending, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := (func(w io.Writer, comments []commentsComment) error {
		if len(comments) == 0 {
			_, err := fmt.Fprintln(w, "No pending review comments.")
			return err
		}
		if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
			return err
		}
		for index, comment := range comments {
			if err := func() error {
				anchor := comment.Anchor
				_, headingErr := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, anchor.FilePath, func() string {
					var sides []string
					if anchor.OldStart > 0 {
						sides = append(sides, func() string {
							var side string = "old"
							if anchor.OldEnd == 0 || anchor.OldEnd == anchor.OldStart {
								return fmt.Sprintf("%s line %d", side, anchor.OldStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.OldStart, anchor.OldEnd)
						}())
					}
					if anchor.NewStart > 0 {
						sides = append(sides, func() string {
							var side string = "new"
							if anchor.NewEnd == 0 || anchor.NewEnd == anchor.NewStart {
								return fmt.Sprintf("%s line %d", side, anchor.NewStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.NewStart, anchor.NewEnd)
						}())
					}
					if len(sides) == 0 {
						return "diff lines"
					}
					return strings.Join(sides, ", ")
				}())
				if headingErr != nil {
					return headingErr
				}
				if err := func() error {
					if len(anchor.QuotedLines) == 0 {
						return nil
					}
					if _, err := fmt.Fprintln(w, "```diff"); err != nil {
						return err
					}
					for _, line := range anchor.QuotedLines {
						if _, err := fmt.Fprintln(w, line); err != nil {
							return err
						}
					}
					_, err := fmt.Fprintln(w, "```")
					return err
				}(); err != nil {
					return err
				}
				_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
				return err
			}(); err != nil {
				return err
			}
		}
		return nil
	})(w, pending); err != nil {
		return err
	}
	ids := make([]string, len(pending))
	for i, comment := range pending {
		ids[i] = comment.ID
	}
	return s.Acknowledge(repository, ids)
}

const (
	commentsMaxCommentBytes = 64 << 10
	commentsMaxPendingBytes = 16 << 20
)

var commentsMessagesBucket = []byte("messages")

type commentsStore struct {
	Path string
}

func (s commentsStore) Add(comment commentsComment) (commentsComment, error) {
	if comment.Repository == "" {
		return commentsComment{}, errors.New("comment requires a repository")
	}
	if err := func() error {
		if len(comment.Body) == 0 {
			return errors.New("comment body is empty")
		}
		if len(comment.Body) > commentsMaxCommentBytes {
			return fmt.Errorf("comment exceeds %d bytes", commentsMaxCommentBytes)
		}
		return nil
	}(); err != nil {
		return commentsComment{}, err
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return commentsComment{}, fmt.Errorf("encode comment: %w", err)
	}
	err = func() error {
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error {
				var id string = comment.ID
				if func() int {
					var total int
					cursor := bucket.Cursor()
					for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
						total += len(value)
					}
					return total
				}()+len(data) > commentsMaxPendingBytes {
					return fmt.Errorf("pending feedback exceeds %d bytes", commentsMaxPendingBytes)
				}
				if bucket.Get([]byte(id)) != nil {
					return errors.New("comment ID already exists")
				}
				return bucket.Put([]byte(id), data)
			}(bucket)
		})
	}()
	return comment, err
}

// List returns pending comments for the repository.
func (s commentsStore) List(repository string) ([]commentsComment, error) {
	var comments []commentsComment
	err := func() error {
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		defer db.Close()
		return db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(commentsMessagesBucket)
			if bucket == nil {
				return nil
			}
			return func(bucket *bolt.Bucket) error {
				return bucket.ForEach(func(_, value []byte) error {
					comment, err := func() (commentsComment, error) {
						var legacy commentsLegacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return commentsComment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment commentsComment
						if err := json.Unmarshal(value, &comment); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return err
					}
					if comment.Repository == repository {
						comments = append(comments, comment)
					}
					return nil
				})
			}(bucket)
		})
	}()
	return comments, err
}

// Update replaces an existing comment in the repository.
func (s commentsStore) Update(comment commentsComment) error {
	if comment.Repository == "" || comment.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := func() error {
		if len(comment.Body) == 0 {
			return errors.New("comment body is empty")
		}
		if len(comment.Body) > commentsMaxCommentBytes {
			return fmt.Errorf("comment exceeds %d bytes", commentsMaxCommentBytes)
		}
		return nil
	}(); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	{
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error {
				key, err := func() ([]byte, error) {
					var repository, id string = comment.Repository, comment.ID
					cursor := bucket.Cursor()
					for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
						comment, err := func() (commentsComment, error) {
							var legacy commentsLegacyRecord
							if err := json.Unmarshal(value, &legacy); err != nil {
								return commentsComment{}, fmt.Errorf("decode comment: %w", err)
							}
							if legacy.Comment != nil {
								return commentsComment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
							}
							var comment commentsComment
							if err := json.Unmarshal(value, &comment); err != nil {
								return commentsComment{}, fmt.Errorf("decode comment: %w", err)
							}
							return comment, nil
						}()
						if err != nil {
							return nil, err
						}
						if comment.Repository == repository && comment.ID == id {
							return append([]byte(nil), key...), nil
						}
					}
					return nil, nil
				}()
				if err != nil {
					return err
				}
				if key == nil {
					return errors.New("comment is no longer in the comments")
				}
				if !bytes.Equal(key, []byte(comment.ID)) {
					if err := bucket.Delete(key); err != nil {
						return err
					}
				}
				return bucket.Put([]byte(comment.ID), data)
			}(bucket)
		})
	}
}

// Delete removes the identified comment from the repository.
func (s commentsStore) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	db, err := func() (*bolt.DB, error) {
		if s.Path == "" {
			return nil, errors.New("comments path is empty")
		}
		if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("create comments directory: %w", err)
		}
		if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("secure comments directory: %w", err)
		}
		db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("open comments: %w", err)
		}
		if err := os.Chmod(s.Path, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("secure comments database: %w", err)
		}
		return db, nil
	}()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
		if err != nil {
			return err
		}
		return func(bucket *bolt.Bucket) error {
			key, err := func() ([]byte, error) {
				cursor := bucket.Cursor()
				for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
					comment, err := func() (commentsComment, error) {
						var legacy commentsLegacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return commentsComment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment commentsComment
						if err := json.Unmarshal(value, &comment); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return nil, err
					}
					if comment.Repository == repository && comment.ID == id {
						return append([]byte(nil), key...), nil
					}
				}
				return nil, nil
			}()
			if err != nil {
				return err
			}
			if key == nil {
				return errors.New("comment is no longer in the comments")
			}
			return bucket.Delete(key)
		}(bucket)
	})
}

// Acknowledge removes the identified comments after their feedback is consumed.

// Preferences contains the persisted UI preferences.

// Load reads preferences from the user configuration directory. Missing files
// use defaults.

// Save atomically replaces the user preferences file.

// navigate applies a diff navigation binding and reports whether it handled the
// key.

// Package comments owns review feedback, its anchors, persistence, and
// delivery.
// WritePending exports feedback and acknowledges it after successful output.
// It has no knowledge of terminal layout or input handling.

// Package commentscreen presents a selectable list of pending comments.

// View owns list focus, scrolling, and comment presentation.

// New creates a comment list view with default terminal dimensions.

// Update preserves focus by comment ID, falling back to the nearest list index
// when the selected comment disappears. Empty lists are valid.

// Resize updates terminal dimensions and keeps the focused comment visible.

// Move shifts focus by delta comments within the list bounds.

// Selected returns the focused comment, or false when the list is empty.

// Click focuses a visible comment at zero-based terminal coordinates.

// Render composes the screen, showing a non-nil error in place of its hints.

// Package patch retrieves the Patch under review from the current working
// directory. Git execution, repository discovery, parsing, and source reads
// stay behind Get. Returned snapshots own their data and are treated as
// read-only.

// Package helpscreen renders keyboard binding descriptions.

// Binding pairs a keyboard shortcut with its help description.

// Render formats bindings with aligned descriptions and keeps the closing hint
// below the available body. The supplied bindings determine the help content.

// Run starts an interactive review of the supplied patch.

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.

// BeginDrag focuses the code under a left-button press and clears any previous
// selection. Coordinates are zero-based terminal cells, including the header.
// Headers, metadata, empty panes, and cells outside the body are ignored.

// DragTo extends a mouse selection within the starting hunk and pane. It does
// nothing without a code-line press, or when the pointer is over invalid code.

// EndDrag leaves a completed selection available for keyboard actions.

// DataDir returns the application data directory, respecting XDG_DATA_HOME.

// Package app coordinates review workflows and terminal UI screens.

// Package keymap recognizes two-key bindings without executing UI actions.

// Sequence defines the accepted keys following a prefix. RetryUnmatched causes
// an invalid continuation to be processed as a fresh key instead of consumed.

// Matcher retains pending input. Copies have independent pending state.

// New builds a matcher from the supplied prefix rules.

// Feed returns a completed binding with keys separated by a space, or an
// unprefixed key. An empty result means pending or consumed input.

// commentStore supplies the persistence operations used by the terminal client.

// commentState pairs stored comments with the revision used to reject stale
// loads.

// Package diffscreen presents patches and owns their interactive browsing
// state.

// Options controls preferred layout and syntax colors. Narrow screens use a
// unified layout while retaining the side-by-side preference.

// Motion identifies navigation operations without exposing display coordinates.

// Navigation motions move focus between lines, pages, files, and panes.

// Alignment places the focused line within the visible body.

// Top, Center, and Bottom select the focused line position in the viewport.

// Direction chooses the order in which search matches are visited.

// Backward and Forward select the search traversal direction.

// View owns focus, selection, search, and scrolling for a patch presentation.
// It has no knowledge of keys, editors, or comment persistence.

// New creates a view with default dimensions of 80 columns and 30 rows.

// Update replaces the patch while preserving meaningful focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.

func (s commentsStore) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	db, err := func() (*bolt.DB, error) {
		if s.Path == "" {
			return nil, errors.New("comments path is empty")
		}
		if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("create comments directory: %w", err)
		}
		if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("secure comments directory: %w", err)
		}
		db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("open comments: %w", err)
		}
		if err := os.Chmod(s.Path, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("secure comments database: %w", err)
		}
		return db, nil
	}()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
		if err != nil {
			return err
		}
		return func(bucket *bolt.Bucket) error {
			keys, err := func() ([][]byte, error) {
				var keys [][]byte
				err := bucket.ForEach(func(key, value []byte) error {
					comment, err := func() (commentsComment, error) {
						var legacy commentsLegacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return commentsComment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment commentsComment
						if err := json.Unmarshal(value, &comment); err != nil {
							return commentsComment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return err
					}
					if comment.Repository != repository {
						return nil
					}
					if _, ok := wanted[comment.ID]; ok {
						keys = append(keys, append([]byte(nil), key...))
					}
					return nil
				})
				return keys, err
			}()
			if err != nil {
				return err
			}
			for _, key := range keys {
				if err := bucket.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}(bucket)
	})
}

type commentsLegacyComment struct {
	Anchor commentsAnchor `json:"anchor"`
	Body   string         `json:"body"`
}

type commentsLegacyRecord struct {
	ID         string                 `json:"id"`
	Repository string                 `json:"repository"`
	CreatedAt  time.Time              `json:"created_at"`
	Comment    *commentsLegacyComment `json:"comment"`
}

type settingsPreferences struct {
	SideBySide bool `json:"side_by_side"`
}

var appNavigationMotions = map[string]diffscreenMotion{
	"j":             diffscreenNextLine,
	"down":          diffscreenNextLine,
	"k":             diffscreenPreviousLine,
	"up":            diffscreenPreviousLine,
	"ctrl+d":        diffscreenNextPage,
	"ctrl+u":        diffscreenPreviousPage,
	"g g":           diffscreenFirstLine,
	"G":             diffscreenLastLine,
	"] f":           diffscreenNextFile,
	"[ f":           diffscreenPreviousFile,
	"ctrl+w h":      diffscreenOldPane,
	"ctrl+w l":      diffscreenNewPane,
	"ctrl+w ctrl+w": diffscreenOtherPane,
}

var appNavigationAlignments = map[string]diffscreenAlignment{
	"z z": diffscreenCenter,
	"z t": diffscreenTop,
	"z b": diffscreenBottom,
}

type commentscreenView struct {
	items         []commentsComment
	row           int
	top           int
	width, height int
}

func (v *commentscreenView) Update(items []commentsComment) {
	selected, ok := v.Selected()
	v.items = append([]commentsComment(nil), items...)
	v.row = min(v.row, max(0, len(items)-1))
	if ok && selected.ID != "" {
		for index, item := range items {
			if item.ID == selected.ID {
				v.row = index
				break
			}
		}
	}
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Resize(width, height int) {
	v.width, v.height = width, height
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Move(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Selected() (commentsComment, bool) {
	if len(v.items) == 0 {
		return commentsComment{}, false
	}
	return v.items[v.row], true
}

func (v *commentscreenView) Click(x, y int) bool {
	height := (func(height int) int { return max(1, height-3) })(v.height)
	row, ok := (func(x, y, width, height int) (int, bool) {
		row := y - 1
		return row, x >= 0 && x < width && row >= 0 &&
			row < (func(height int) int { return max(1, height-3) })(height) && y < height-2
	})(x, y, v.width, v.height)
	if !ok {
		return false
	}
	row += min(max(0, v.top), max(0, len(v.items)-height))
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *commentscreenView) Render(err error) string {
	pending := commentscreenMutedStyle.Render(fmt.Sprintf("%d pending", len(v.items)))
	header := commentscreenTitleStyle.Render("comments") + "  " + pending
	height := (func(height int) int { return max(1, height-3) })(v.height)
	body := make([]string, 0, height)
	if len(v.items) == 0 {
		body = make([]string, height)
		body[min(1, height-1)] = commentscreenMutedStyle.Render("No pending comments.")
	} else {
		start := min(max(0, v.top), max(0, len(v.items)-height))
		end := min(len(v.items), start+height)
		for index := start; index < end; index++ {
			body = append(body, (func(index int) string {
				comment := v.items[index]
				prefix, style := "  ", lipgloss.NewStyle()
				if index == v.row {
					prefix, style = "> ", lipgloss.NewStyle().Reverse(true)
				}
				location := comment.Anchor.FilePath
				if comment.Anchor.NewStart > 0 {
					location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
				} else if comment.Anchor.OldStart > 0 {
					location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
				}
				text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
				line := ansi.Truncate(
					fmt.Sprintf("%s%s  %s", prefix, location, text),
					max(20, v.width),
					"",
				)
				return style.Width(max(20, v.width)).Render(line)
			})(index))
		}
	}
	footer := commentscreenMutedStyle.Render(
		"j/k move  Enter/e edit  D delete  Esc/q return",
	)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), "\n", " ")
		footer = commentscreenErrorStyle.Render(ansi.Truncate(message, max(20, v.width), ""))
	}
	return (func(header string, body []string, footer string, height int) string {
		bodyHeight := (func(height int) int { return max(1, height-3) })(height)
		lines := make([]string,

			// Add validates and stores a comment, assigning an ID and timestamp when
			// absent.
			0, bodyHeight+3)
		lines = append(lines, header)
		lines = append(lines, body[:min(len(body), bodyHeight)]...)
		for len(lines) < bodyHeight+1 {
			lines = append(lines, "")
		}
		lines = append(lines, footer, "")
		return strings.Join(lines, "\n")
	})(header, body, footer, v.height)
}

var (
	commentscreenTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	commentscreenMutedStyle = lipgloss.NewStyle().Faint(true)
	commentscreenErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)

type helpscreenBinding struct {
	Keys        string
	Description string
}

const patchMaxFileBytes = 2 << 20

type patchRunner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type patchExecRunner struct{}

func (patchExecRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf(
			"git %s: %s",
			strings.Join(args, " "),
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

type patchSourceReader func(context.Context, patchRunner, string, string) string

func (v *diffscreenView) BeginDrag(x, y int) bool {
	v.EndDrag()
	cursor, ok := func() (diffscreenDiffCursor, bool) {
		row, ok := (func(x, y, width, height int) (int, bool) {
			row := y - 1
			return row, x >= 0 && x < width && row >= 0 &&
				row < (func(height int) int { return max(1, height-3) })(height) && y < height-2
		})(x, y, v.width, v.height)
		if !ok {
			return diffscreenDiffCursor{}, false
		}
		if func(v *diffscreenDiffView, inlineLocalTop1 int, viewportHeight int) bool {
			return viewportHeight > 1 && inlineLocalTop1 >= 0 && inlineLocalTop1 < len(v.rows) && v.rows[inlineLocalTop1].kind != diffscreenFileRow
		}(v.view, v.viewport.top, v.viewport.Height) {
			row--
		}
		if row < 0 || row >= func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				height--
			}
			return max(1, height)
		}(v.view, v.viewport) {
			return diffscreenDiffCursor{}, false
		}
		pane := diffscreenRight
		if v.view.split {
			leftWidth, _ := func() (inlineLocalLeft2, inlineLocalRight1 int) {
				inlineLocalLeft2 = max(20, (v.viewport.Width-3)/2)
				return inlineLocalLeft2, max(20, v.viewport.Width-3-inlineLocalLeft2)
			}()
			switch {
			case x < leftWidth:
				pane = diffscreenLeft
			case x < leftWidth+3:
				return diffscreenDiffCursor{}, false
			default:
				pane = diffscreenRight
			}
		}
		return func(v *diffscreenDiffView, y int) (diffscreenDiffCursor, bool) {
			cursor := diffscreenDiffCursor{row: y, pane: pane}
			return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}(v.view, v.viewport.top+row)
	}()
	if !ok {
		return false
	}
	v.ClearSelection()
	v.cursor = cursor
	v.drag = &cursor
	return true
}

func (v *diffscreenView) DragTo(x, y int) {
	if v.drag == nil {
		return
	}
	cursor, ok := func() (diffscreenDiffCursor, bool) {
		row, ok := (func(x, y, width, height int) (int, bool) {
			row := y - 1
			return row, x >= 0 && x < width && row >= 0 &&
				row < (func(height int) int { return max(1, height-3) })(height) && y < height-2
		})(x, y, v.width, v.height)
		if !ok {
			return diffscreenDiffCursor{}, false
		}
		if func(v *diffscreenDiffView, inlineLocalTop2 int, viewportHeight int) bool {
			return viewportHeight > 1 && inlineLocalTop2 >= 0 && inlineLocalTop2 < len(v.rows) && v.rows[inlineLocalTop2].kind != diffscreenFileRow
		}(v.view, v.viewport.top, v.viewport.Height) {
			row--
		}
		if row < 0 || row >= func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				height--
			}
			return max(1, height)
		}(v.view, v.viewport) {
			return diffscreenDiffCursor{}, false
		}
		pane := diffscreenRight
		if v.view.split {
			leftWidth, _ := func() (inlineLocalLeft1, inlineLocalRight2 int) {
				inlineLocalLeft1 = max(20, (v.viewport.Width-3)/2)
				return inlineLocalLeft1, max(20, v.viewport.Width-3-inlineLocalLeft1)
			}()
			switch {
			case x < leftWidth:
				pane = diffscreenLeft
			case x < leftWidth+3:
				return diffscreenDiffCursor{}, false
			default:
				pane = diffscreenRight
			}
		}
		return func(v *diffscreenDiffView, y int) (diffscreenDiffCursor, bool) {
			cursor := diffscreenDiffCursor{row: y, pane: pane}
			return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}(v.view, v.viewport.top+row)
	}()
	if !ok || cursor.pane != v.drag.pane {
		return
	}
	if cursor == *v.drag && v.selection == nil {
		return
	}
	selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
		if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
			var (
				current diffscreenEntry    = v.rows[selection.First.row]
				pane    diffscreenDiffPane = selection.First.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := v.rows[selection.First.row]
		last := v.rows[cursor.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = cursor
		return selection, true
	}(v.view, func() diffscreenDiffSelection {
		var cursor diffscreenDiffCursor = *v.drag
		return diffscreenDiffSelection{First: cursor, Last: cursor}
	}())
	if !ok {
		return
	}
	v.cursor, v.selection = cursor, &selection
}

func (v *diffscreenView) EndDrag() { v.drag = nil }

const commentsAppName = "review-my-slop"

type keymapSequence struct {
	Prefix         string
	Keys           []string
	RetryUnmatched bool
}

type keymapMatcher struct {
	sequences map[string]keymapSequence
	pending   string
}

func (m *keymapMatcher) Feed(key string) string {
	if m.pending != "" {
		prefix := m.pending
		m.pending = ""
		sequence := m.sequences[prefix]
		if slices.Contains(sequence.Keys, key) {
			return prefix + " " + key
		}
		if !sequence.RetryUnmatched {
			return ""
		}
	}
	if _, ok := m.sequences[key]; ok {
		m.pending = key
		return ""
	}
	return key
}

type appSaveCommentFunc func(commentsComment, patchPatch,

) (commentsComment, error)
type appDeleteCommentFunc func(commentsComment, patchPatch) error
type appLoadCommentsFunc func() ([]commentsComment, error)
type appRefreshDiffFunc func(patchKind) (patchPatch, error)
type appSaveSideBySideFunc func(bool) error

type appCommentStore interface {
	Add(commentsComment) (commentsComment, error)
	List(string) ([]commentsComment, error)
	Update(commentsComment) error
	Delete(string, string) error
}

type appSize struct {
	Width  int
	Height int
}

type appInitialLayout struct {
	SideBySide     bool
	SaveSideBySide appSaveSideBySideFunc
	size           appSize
}

type appRefreshDiffMsg struct {
	patch patchPatch
	kind  patchKind
	err   error
}

type appCommentEditorFinishedMsg struct {
	body string
	err  error
}
type appCommentsLoadedMsg struct {
	comments []commentsComment
	revision uint64
	err      error
}
type appSourceEditorFinishedMsg struct{ err error }

type appMode uint8

const (
	appModeBrowse appMode = iota
	appModeComments
	appModeHelp
	appModeSearch
)

const appHorizontalScrollStep = 4
const appVerticalScrollStep = 3

var appDefaultSize = appSize{Width: 80, Height: 30}

type appCommentState struct {
	items    []commentsComment
	revision uint64
}

type appCommentEdit struct {
	body   string
	index  int
	anchor commentsAnchor
}

type appModel struct {
	ctx context.Context

	diffView    *diffscreenView
	commentView *commentscreenView
	width       int
	height      int
	mode        appMode
	keys        keymapMatcher

	currentPatch patchPatch
	kind         patchKind
	diffOptions  diffscreenOptions
	comments     appCommentState
	edit         appCommentEdit

	save       appSaveCommentFunc
	delete     appDeleteCommentFunc
	load       appLoadCommentsFunc
	refresh    appRefreshDiffFunc
	saveLayout appSaveSideBySideFunc
	err        error
	quitting   bool
}

func (appModel) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diffView.Configure(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)

	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		var m appModel = m
		name := msg.String()
		if m.mode == appModeComments {
			var m appModel = m
			m.err = nil
			switch name {
			case "esc", "C", "q":
				m.mode = appModeBrowse
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "j", "down":
				m.commentView.Move(1)
			case "k", "up":
				m.commentView.Move(-1)
			case "enter", "e":
				var m appModel = m
				selected, ok := m.commentView.Selected()
				if !ok {
					return m, nil
				}
				m.edit.index = func() int {
					for index, item := range m.comments.items {
						if selected.ID != "" && item.ID == selected.ID || selected.ID == "" && item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath {
							return index
						}
					}
					return -1
				}()
				if m.edit.index < 0 {
					return m, nil
				}
				m.edit.body = m.comments.items[m.edit.index].Body
				m.edit.anchor = m.comments.items[m.edit.index].Anchor
				cmd, err := (func(
					ctx context.Context,
					body string,
					anchor commentsAnchor,
					onComplete func(string, error) tea.Msg,
				) (tea.Cmd, error) {
					command, err := (func() (string, error) {
						command := strings.TrimSpace(os.Getenv("EDITOR"))
						if command == "" {
							return "", errors.New("$EDITOR is not set")
						}
						return command, nil
					})()
					if err != nil {
						return nil, err
					}
					adapter, complete := (func(
						ctx context.Context,
						command, body string,
						anchor commentsAnchor,
						onComplete func(string, error) tea.Msg,
					) (*editorCommandAdapter, tea.ExecCallback) {
						adapter := &editorCommandAdapter{}
						adapter.run = func() error {
							path, err := func() (string, error) {
								file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
								if err != nil {
									return "", fmt.Errorf("create comment file: %w", err)
								}
								path := file.Name()
								if _, err := file.WriteString(func() string {
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
								return err
							}
							defer os.Remove(path)
							editorCommand := exec.CommandContext(ctx, "sh", "-c", command+" "+(func(value string) string {
								return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
							})(path))
							if err := func() error {
								editorCommand.Stdin, editorCommand.Stdout, editorCommand.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
								return editorCommand.Run()
							}(); err != nil {
								return fmt.Errorf("editor: %w", err)
							}
							adapter.body, err = func() (string, error) {
								body, err := os.ReadFile(path)
								if err != nil {
									return "", fmt.Errorf("read comment file: %w", err)
								}
								return func() string {
									if len(anchor.QuotedLines) == 0 {
										return string(body)
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
							return err
						}
						return adapter, func(err error) tea.Msg {
							return onComplete(adapter.body, err)
						}
					})(ctx, command, body, anchor, onComplete)
					return tea.Exec(adapter, complete), nil
				})(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg { return appCommentEditorFinishedMsg{body: body, err: err} })
				if err != nil {
					m.err = err
					m.edit = appCommentEdit{index: -1}
					return m, nil
				}
				return m, cmd
			case "D":
				if selected, ok := m.commentView.Selected(); ok {
					func(m *appModel, index int) {
						if index < 0 || index >= len(m.comments.items) {
							return
						}
						if m.delete == nil {
							m.err = errors.New("comment storage is unavailable")
							return
						}
						if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
							m.err = err
							return
						}
						m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
						m.commentView.Update(m.comments.items)
						m.comments.revision++
						m.err = nil
					}(&m, func() int {
						for index, item := range m.comments.items {
							if selected.ID != "" && item.ID == selected.ID || selected.ID == "" && item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath {
								return index
							}
						}
						return -1
					}())
				}
			default:
				return m, nil
			}
			return m, nil
		}
		if m.mode == appModeHelp {
			if name == "esc" || name == "?" || name == "q" {
				m.mode = appModeBrowse
			}
			return m, nil
		}
		if m.mode == appModeSearch {
			var m appModel = m
			switch name {
			case "esc":
				m.diffView.CancelSearch()
				m.mode = appModeBrowse
			case "enter":
				m.diffView.AcceptSearch()
				m.mode = appModeBrowse
			case "backspace":
				m.diffView.BackspaceSearch()
			default:
				if msg.Text != "" {
					m.diffView.InsertSearch(msg.Text)
				}
			}
			return m, nil
		}
		{
			var (
				m    appModel = m
				name string   = name
			)
			m.err = nil
			name = m.keys.Feed(name)
			if func() bool {
				var m *appModel = &m
				if motion, ok := appNavigationMotions[name]; ok {
					m.diffView.Move(motion)
					return true
				}
				if alignment, ok := appNavigationAlignments[name]; ok {
					m.diffView.Align(alignment)
					return true
				}
				switch name {
				case "n":
					m.diffView.Find(diffscreenForward)
				case "N":
					m.diffView.Find(diffscreenBackward)
				case "h", "left":
					m.diffView.ScrollHorizontal(-appHorizontalScrollStep)
				case "l", "right":
					m.diffView.ScrollHorizontal(appHorizontalScrollStep)
				case "0":
					m.diffView.ScrollHorizontal(-int(^uint(0) >> 1))
				case "$":
					m.diffView.ScrollHorizontal(int(^uint(0) >> 1))
				default:
					return false
				}
				return true
			}() {
				return m, nil
			}
			switch name {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "c":
				var m appModel = m
				cmd, err := func() (tea.Cmd, error) {
					file, lines, ok := m.diffView.Selected()
					if !ok {
						return nil, errors.New("select code lines before commenting")
					}
					anchor := func() commentsAnchor {
						path := file.NewPath
						if path == "" {
							path = file.OldPath
						}
						anchor := commentsAnchor{FilePath: path}
						for _, line := range lines {
							prefix := " "
							if line.Kind == patchAddition {
								prefix = "+"
							}
							if line.Kind == patchDeletion {
								prefix = "-"
							}
							anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
							func() {
								var inlineLocalStart1, end *int = &anchor.OldStart, &anchor.OldEnd
								if int(line.OldNumber) == 0 {
									return
								}
								if *inlineLocalStart1 == 0 || int(line.OldNumber) < *inlineLocalStart1 {
									*inlineLocalStart1 = int(line.OldNumber)
								}
								if int(line.OldNumber) > *end {
									*end = int(line.OldNumber)
								}
							}()
							func() {
								var inlineLocalStart2, end *int = &anchor.NewStart, &anchor.NewEnd
								if int(line.NewNumber) == 0 {
									return
								}
								if *inlineLocalStart2 == 0 || int(line.NewNumber) < *inlineLocalStart2 {
									*inlineLocalStart2 = int(line.NewNumber)
								}
								if int(line.NewNumber) > *end {
									*end = int(line.NewNumber)
								}
							}()
						}
						return anchor
					}()
					m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
					cmd, err := (func(
						ctx context.Context,
						body string,
						anchor commentsAnchor,
						onComplete func(string, error) tea.Msg,
					) (tea.Cmd, error) {
						command, err := (func() (string, error) {
							command := strings.TrimSpace(os.Getenv("EDITOR"))
							if command == "" {
								return "", errors.New("$EDITOR is not set")
							}
							return command, nil
						})()
						if err != nil {
							return nil, err
						}
						adapter, complete := (func(
							ctx context.Context,
							command, body string,
							anchor commentsAnchor,
							onComplete func(string, error) tea.Msg,
						) (*editorCommandAdapter, tea.ExecCallback) {
							adapter := &editorCommandAdapter{}
							adapter.run = func() error {
								path, err := func() (string, error) {
									file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
									if err != nil {
										return "", fmt.Errorf("create comment file: %w", err)
									}
									path := file.Name()
									if _, err := file.WriteString(func() string {
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
									return err
								}
								defer os.Remove(path)
								editorCommand := exec.CommandContext(ctx, "sh", "-c", command+" "+(func(value string) string {
									return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
								})(path))
								if err := func() error {
									editorCommand.Stdin, editorCommand.Stdout, editorCommand.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
									return editorCommand.Run()
								}(); err != nil {
									return fmt.Errorf("editor: %w", err)
								}
								adapter.body, err = func() (string, error) {
									body, err := os.ReadFile(path)
									if err != nil {
										return "", fmt.Errorf("read comment file: %w", err)
									}
									return func() string {
										if len(anchor.QuotedLines) == 0 {
											return string(body)
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
								return err
							}
							return adapter, func(err error) tea.Msg {
								return onComplete(adapter.body, err)
							}
						})(ctx, command, body, anchor, onComplete)
						return tea.Exec(adapter, complete), nil
					})(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg { return appCommentEditorFinishedMsg{body: body, err: err} })
					if err != nil {
						m.edit = appCommentEdit{index: -1}
						return nil, err
					}
					return cmd, nil
				}()
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, cmd
			case "e":
				var m appModel = m
				cmd, err := func() (tea.Cmd, error) {
					path, number, err := func() (string, int, error) {
						file, line, ok := m.diffView.Current()
						if !ok {
							return "", 0, errors.New("select a code line to open in $EDITOR")
						}
						path, number := file.NewPath, line.NewNumber
						if path == "" || path == "/dev/null" {
							path = file.OldPath
						}
						if number == 0 {
							number = line.OldNumber
						}
						if path == "" || path == "/dev/null" || number < 1 {
							return "", 0, errors.New("current line has no editable working-tree location")
						}
						if !filepath.IsAbs(path) {
							path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
						}
						return path, int(number), nil
					}()
					if err != nil {
						return nil, err
					}
					return (func(
						ctx context.Context,
						path string,
						line int,
						onComplete func(error) tea.Msg,
					) (tea.Cmd, error) {
						command, err := (func() (string, error) {
							command := strings.TrimSpace(os.Getenv("EDITOR"))
							if command == "" {
								return "", errors.New("$EDITOR is not set")
							}
							return command, nil
						})()
						if err != nil {
							return nil, err
						}
						adapter := &editorCommandAdapter{}
						adapter.run = func() error {
							var cmd *exec.Cmd = exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(line)+" "+(func(value string) string {
								return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
							})(path))
							cmd.Stdin, cmd.Stdout, cmd.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
							return cmd.Run()
						}
						return tea.Exec(adapter, onComplete), nil
					})(m.ctx, path, number, func(err error) tea.Msg { return appSourceEditorFinishedMsg{err: err} })
				}()
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, cmd
			case "R":
				return m, func() tea.Cmd {
					if m.refresh == nil {
						return nil
					}
					kind := m.kind
					return func() tea.Msg { p, err := m.refresh(kind); return appRefreshDiffMsg{patch: p, kind: kind, err: err} }
				}()
			case "tab":
				var m appModel = m
				if m.currentPatch.Branch == "" {
					return m, nil
				}
				if m.kind == patchUnstaged {
					m.kind = patchBranch
				} else {
					m.kind = patchUnstaged
				}
				m.diffView.ClearSelection()
				return m, func() tea.Cmd {
					if m.refresh == nil {
						return nil
					}
					kind := m.kind
					return func() tea.Msg { p, err := m.refresh(kind); return appRefreshDiffMsg{patch: p, kind: kind, err: err} }
				}()
			default:
				var m appModel = m
				switch name {
				case "?":
					m.mode = appModeHelp
				case "/":
					m.mode = appModeSearch
					m.diffView.BeginSearch()
				case "v":
					m.diffView.ToggleSelection()
				case "esc":
					m.diffView.ClearSelection()
				case "C":
					m.mode = appModeComments
					m.commentView.Update(m.comments.items)
					return m, func() tea.Cmd {
						if m.load == nil {
							return nil
						}
						revision := m.comments.revision
						return func() tea.Msg {
							inlineLocalComments1, err := m.load()
							return appCommentsLoadedMsg{comments: inlineLocalComments1, revision: revision, err: err}
						}
					}()
				case "t":
					enabled := !m.diffOptions.SideBySide
					m.diffOptions.SideBySide = enabled
					m.diffView.Configure(m.diffOptions)
					if m.saveLayout != nil {
						if err := m.saveLayout(m.diffOptions.SideBySide); err != nil {
							m.err = fmt.Errorf("save side-by-side preference: %w", err)
						}
					}
				default:
					return m, nil
				}
				return m, nil
			}
		}
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		func() {
			var m *appModel = &m
			switch msg := msg.(type) {
			case tea.MouseClickMsg:
				func() {
					if msg.Button != tea.MouseLeft {
						return
					}
					switch m.mode {
					case appModeBrowse:
						m.diffView.BeginDrag(msg.X, msg.Y)
					case appModeComments:
						m.commentView.Click(msg.X, msg.Y)
					default:
						return
					}
				}()
			case tea.MouseMotionMsg:
				if msg.Button == tea.MouseLeft && m.mode == appModeBrowse {
					m.diffView.DragTo(msg.X, msg.Y)
				}
			case tea.MouseReleaseMsg:
				func() {
					if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
						return
					}
					if m.mode == appModeBrowse {
						m.diffView.DragTo(msg.X, msg.Y)
					}
					m.diffView.EndDrag()
				}()
			case tea.MouseWheelMsg:
				func() {
					m.diffView.EndDrag()
					if m.mode != appModeBrowse && m.mode != appModeSearch {
						return
					}
					switch msg.Button {
					case tea.MouseWheelUp:
						m.diffView.ScrollVertical(-appVerticalScrollStep)
					case tea.MouseWheelDown:
						m.diffView.ScrollVertical(appVerticalScrollStep)
					case tea.MouseWheelLeft:
						m.diffView.ScrollHorizontal(-appHorizontalScrollStep)
					case tea.MouseWheelRight:
						m.diffView.ScrollHorizontal(appHorizontalScrollStep)
					default:
						return
					}
				}()
			default:
				return
			}
		}()
	default:
		var m appModel = m
		switch msg := msg.(type) {
		case appCommentEditorFinishedMsg:
			func() {
				var m *appModel = &m
				if msg.err != nil {
					m.err = msg.err
					m.edit = appCommentEdit{index: -1}
					return
				}
				m.edit.body = msg.body
				func() {
					body := strings.TrimSpace(m.edit.body)
					if body == "" {
						if m.edit.index >= 0 {
							func() {
								var index int = m.edit.index
								if index < 0 || index >= len(m.comments.items) {
									return
								}
								if m.delete == nil {
									m.err = errors.New("comment storage is unavailable")
									return
								}
								if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
									m.err = err
									return
								}
								m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
								m.commentView.Update(m.comments.items)
								m.comments.revision++
								m.err = nil
							}()
						}
						m.edit = appCommentEdit{index: -1}
						m.diffView.ClearSelection()
						return
					}
					if m.save == nil {
						m.err = errors.New("comment storage is unavailable")
						m.edit = appCommentEdit{index: -1}
						return
					}
					var comment commentsComment
					if m.edit.index >= 0 {
						comment = m.comments.items[m.edit.index]
						comment.Body = body
					} else {
						comment = commentsComment{Anchor: m.edit.anchor, Body: body}
					}
					saved, err := m.save(comment, m.currentPatch)
					if err != nil {
						m.err = err
						m.edit = appCommentEdit{index: -1}
						return
					}
					if m.edit.index >= 0 {
						m.comments.items[m.edit.index] = saved
					} else {
						m.comments.items = append(m.comments.items, saved)
						m.commentView.Update(m.comments.items)
						m.commentView.Move(len(m.comments.items))
					}
					m.comments.revision++
					m.commentView.Update(m.comments.items)
					m.edit = appCommentEdit{index: -1}
					m.err = nil
					m.diffView.ClearSelection()
				}()
			}()
		case appCommentsLoadedMsg:
			func() {
				if msg.revision != m.comments.revision {
					return
				}
				if msg.err != nil {
					m.err = fmt.Errorf("refresh comments: %w", msg.err)
					return
				}
				m.comments.items = msg.comments
				m.commentView.Update(m.comments.items)
				m.err = nil
			}()
		case appSourceEditorFinishedMsg:
			if msg.err != nil {
				m.err = fmt.Errorf("editor: %w", msg.err)
				break
			}
			return m, func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return appRefreshDiffMsg{patch: p, kind: kind, err: err} }
			}()
		case tea.FocusMsg:
			return m, func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return appRefreshDiffMsg{patch: p, kind: kind, err: err} }
			}()
		case appRefreshDiffMsg:
			func() {
				if msg.kind != m.kind {
					return
				}
				if msg.err != nil {
					m.err = fmt.Errorf("refresh diff: %w", msg.err)
					return
				}
				m.currentPatch = msg.patch
				m.diffView.Update(msg.patch)
				m.err = nil
			}()
		default:
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

type diffscreenOptions struct {
	SideBySide bool
	Dark       bool
}

type diffscreenMotion uint8

const (
	diffscreenPreviousLine diffscreenMotion = iota
	diffscreenNextLine
	diffscreenPreviousPage
	diffscreenNextPage
	diffscreenFirstLine
	diffscreenLastLine
	diffscreenPreviousFile
	diffscreenNextFile
	diffscreenOldPane
	diffscreenNewPane
	diffscreenOtherPane
)

type diffscreenAlignment uint8

const (
	diffscreenTop diffscreenAlignment = iota
	diffscreenCenter
	diffscreenBottom
)

type diffscreenDirection int8

const (
	diffscreenBackward diffscreenDirection = -1
	diffscreenForward  diffscreenDirection = 1
)

type diffscreenView struct {
	view          *diffscreenDiffView
	cursor        diffscreenDiffCursor
	viewport      diffscreenDiffViewport
	selection     *diffscreenDiffSelection
	drag          *diffscreenDiffCursor
	options       diffscreenOptions
	width, height int
	search        diffscreenSearchState
}

type diffscreenSearchState struct {
	active           bool
	query, term      string
	from             diffscreenDiffCursor
	miss, repeatMiss bool
}

func (v *diffscreenView) Update(p patchPatch) {
	v.EndDrag()
	v.search.repeatMiss = false
	var next *diffscreenDiffView = func() *diffscreenDiffView {
		{
			var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
			v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
			for fileIndex, file := range p.Files {
				var file patchFile = file
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
				for _, metadata := range file.Metadata {
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
				}
				highlighted := func() diffscreenPair {
					var file *patchFile = &file
					theme := diffscreenLightTheme
					if v.dark {
						theme = diffscreenDarkTheme
					}
					return diffscreenPair{Old: func() []string {
						var filename, source, theme string = file.OldPath, file.OldSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}(), New: func() []string {
						var filename, source, theme string = file.NewPath, file.NewSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}()}
				}()
				for hunkIndex, hunk := range file.Hunks {
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
						if strings.HasPrefix(hunk.Header, "@@") {
							return hunk.Header
						}
						return "@@ " + hunk.Header
					}()})
					for _, indices := range func() [][2]int {
						rows := make([][2]int, 0, len(hunk.Lines))
						for index := 0; index < len(hunk.Lines); {
							if !v.split || hunk.Lines[index].Kind == patchContext {
								rows = append(rows, [2]int{index, index})
								index++
								continue
							}
							if hunk.Lines[index].Kind == patchAddition {
								rows = append(rows, [2]int{-1, index})
								index++
								continue
							}
							addedStart := func() int {
								var start int = index
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
									start++
								}
								return start
							}()
							end := func() int {
								var start int = addedStart
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
									start++
								}
								return start
							}()
							rows = append(rows, func() [][2]int {
								count := max(addedStart-index, end-addedStart)
								rows := make([][2]int, 0, count)
								for offset := 0; offset < count; offset++ {
									inlineLocalPair1 := [2]int{-1, -1}
									if index+offset < addedStart {
										inlineLocalPair1[0] = index + offset
									}
									if addedStart+offset < end {
										inlineLocalPair1[1] = addedStart + offset
									}
									rows = append(rows, inlineLocalPair1)
								}
								return rows
							}()...)
							index = end
						}
						return rows
					}() {
						row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
						func() {
							var row *diffscreenEntry = &row
							if !v.split {
								row.text = func() string {
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
								return
							}
							if row.leftLine >= 0 {
								row.left = func() string {
									var line patchLine = hunk.Lines[row.leftLine]
									if line.Kind == patchDeletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
							if row.rightLine >= 0 {
								row.right = func() string {
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
						}()
						v.rows = append(v.rows, row)
					}
				}
			}
			return v
		}
	}()
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := func() diffscreenCursorIdentity {
		var cursor diffscreenDiffCursor = v.cursor
		if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
			var current diffscreenEntry = old.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenCursorIdentity{}
		}
		row := old.rows[cursor.row]
		file := old.patch.Files[row.file]
		hunk := file.Hunks[row.hunk]
		line := hunk.Lines[func() int {
			if row.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || cursor.pane == diffscreenRight {
				return row.rightLine
			}
			return row.leftLine
		}()]
		return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
	}()
	origin := func() diffscreenCursorIdentity {
		if !(v.search.from.row >= 0 && v.search.from.row < len(old.rows) && func() int {
			var current diffscreenEntry = old.rows[v.search.from.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || v.search.from.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenCursorIdentity{}
		}
		row := old.rows[v.search.from.row]
		file := old.patch.Files[row.file]
		hunk := file.Hunks[row.hunk]
		line := hunk.Lines[func() int {
			if row.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || v.search.from.pane == diffscreenRight {
				return row.rightLine
			}
			return row.leftLine
		}()]
		return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: v.search.from, valid: true}
	}()
	v.viewport = func() diffscreenDiffViewport {
		var width, height int = v.viewport.Width, v.viewport.Height
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, width), max(1, height)
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(next.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	v.viewport = func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = v.viewport
			columns  int                    = leftColumn
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if next.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range next.rows {
				if current.kind != diffscreenLineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(next.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	translated, ok := func() (diffscreenDiffCursor, bool) {
		if !cursor.valid {
			return diffscreenDiffCursor{}, false
		}
		best := diffscreenCursorMatch{}
		for y := range next.rows {
			candidate := func() diffscreenCursorMatch {
				current := next.rows[y]
				if !(current.hunk >= 0 && func() bool {
					var candidate patchFile = next.patch.Files[current.file]
					return candidate.OldPath != "" && candidate.OldPath == cursor.file.OldPath || candidate.NewPath != "" && candidate.NewPath == cursor.file.NewPath
				}()) {
					return diffscreenCursorMatch{}
				}
				best := diffscreenCursorMatch{}
				for _, pane := range [2]diffscreenDiffPane{cursor.cursor.pane, func() diffscreenDiffPane {
					if cursor.cursor.pane == diffscreenLeft {
						return diffscreenRight
					}
					return diffscreenLeft
				}()} {
					candidate, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patchLine, bool) {
						if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[candidate.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchLine{}, false
						}
						current := next.rows[candidate.row]
						return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
						if line.Kind != cursor.line.Kind {
							return 1
						}
						if line.OldNumber == cursor.line.OldNumber && line.NewNumber == cursor.line.NewNumber {
							return 3
						}
						if line.Text == cursor.line.Text {
							return 2
						}
						return 1
					}(), distance: func() int {
						var rowDistance int = y - cursor.cursor.row
						if line.NewNumber > 0 && cursor.line.NewNumber > 0 {
							if int(line.NewNumber-cursor.line.NewNumber) < 0 {
								return -int(line.NewNumber - cursor.line.NewNumber)
							}
							return int(line.NewNumber - cursor.line.NewNumber)
						}
						if line.OldNumber > 0 && cursor.line.OldNumber > 0 {
							if int(line.OldNumber-cursor.line.OldNumber) < 0 {
								return -int(line.OldNumber - cursor.line.OldNumber)
							}
							return int(line.OldNumber - cursor.line.OldNumber)
						}
						if rowDistance < 0 {
							return -rowDistance
						}
						return rowDistance
					}()}
					if match.rank == 3 {
						return match
					}
					best = func() diffscreenCursorMatch {
						if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
							return match
						}
						return best
					}()
				}
				return best
			}()
			if candidate.rank == 3 {
				return candidate.cursor, true
			}
			if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
				best = candidate
			}
		}
		return best.cursor, best.rank > 0
	}()
	if !ok {
		translated, _ = func() (diffscreenDiffCursor, bool) {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(diffscreenPrivateForward)
			for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
						var current diffscreenEntry = next.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !next.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}()
	}
	v.cursor = translated
	v.selection = func() *diffscreenDiffSelection {
		var selection *diffscreenDiffSelection = v.selection
		if selection == nil {
			return nil
		}
		first := func() diffscreenCursorIdentity {
			var cursor diffscreenDiffCursor = selection.First
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		last := func() diffscreenCursorIdentity {
			var cursor diffscreenDiffCursor = selection.Last
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		if !first.valid || !last.valid {
			return nil
		}
		translatedFirst, firstOK := func() (diffscreenDiffCursor, bool) {
			if !first.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == first.file.OldPath || candidate.NewPath != "" && candidate.NewPath == first.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{first.cursor.pane, func() diffscreenDiffPane {
						if first.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != first.line.Kind {
								return 1
							}
							if line.OldNumber == first.line.OldNumber && line.NewNumber == first.line.NewNumber {
								return 3
							}
							if line.Text == first.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - first.cursor.row
							if line.NewNumber > 0 && first.line.NewNumber > 0 {
								if int(line.NewNumber-first.line.NewNumber) < 0 {
									return -int(line.NewNumber - first.line.NewNumber)
								}
								return int(line.NewNumber - first.line.NewNumber)
							}
							if line.OldNumber > 0 && first.line.OldNumber > 0 {
								if int(line.OldNumber-first.line.OldNumber) < 0 {
									return -int(line.OldNumber - first.line.OldNumber)
								}
								return int(line.OldNumber - first.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() diffscreenCursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}()
		translatedLast, lastOK := func() (diffscreenDiffCursor, bool) {
			if !last.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == last.file.OldPath || candidate.NewPath != "" && candidate.NewPath == last.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{last.cursor.pane, func() diffscreenDiffPane {
						if last.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != last.line.Kind {
								return 1
							}
							if line.OldNumber == last.line.OldNumber && line.NewNumber == last.line.NewNumber {
								return 3
							}
							if line.Text == last.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - last.cursor.row
							if line.NewNumber > 0 && last.line.NewNumber > 0 {
								if int(line.NewNumber-last.line.NewNumber) < 0 {
									return -int(line.NewNumber - last.line.NewNumber)
								}
								return int(line.NewNumber - last.line.NewNumber)
							}
							if line.OldNumber > 0 && last.line.OldNumber > 0 {
								if int(line.OldNumber-last.line.OldNumber) < 0 {
									return -int(line.OldNumber - last.line.OldNumber)
								}
								return int(line.OldNumber - last.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() diffscreenCursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}()
		if !firstOK || !lastOK || !(first.file.OldPath != "" && first.file.OldPath == last.file.OldPath || first.file.NewPath != "" && first.file.NewPath == last.file.NewPath) || first.hunk.Header != last.hunk.Header {
			return nil
		}
		translated := diffscreenDiffSelection{First: translatedFirst, Last: translatedFirst}
		translated, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = translated
			if !(selection.First.row >= 0 && selection.First.row < len(next.rows) && func() int {
				var (
					current diffscreenEntry    = next.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(translatedLast.row >= 0 && translatedLast.row < len(next.rows) && func() int {
				var current diffscreenEntry = next.rows[translatedLast.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || translatedLast.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := next.rows[selection.First.row]
			last := next.rows[translatedLast.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = translatedLast
			return selection, true
		}()
		if !ok {
			return nil
		}
		return &translated
	}()
	v.view = next
	if func() bool {
		var cursor diffscreenDiffCursor = v.cursor
		return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current diffscreenEntry = next.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0
	}() {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = func() diffscreenDiffViewport {
			var (
				viewport diffscreenDiffViewport = v.viewport
				cursor   diffscreenDiffCursor   = v.cursor
			)
			if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
				var current diffscreenEntry = next.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if next.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range next.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if next.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range next.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(next.rows) && next.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if next.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range next.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}()
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := func() (diffscreenDiffCursor, bool) {
			if !origin.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == origin.file.OldPath || candidate.NewPath != "" && candidate.NewPath == origin.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{origin.cursor.pane, func() diffscreenDiffPane {
						if origin.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != origin.line.Kind {
								return 1
							}
							if line.OldNumber == origin.line.OldNumber && line.NewNumber == origin.line.NewNumber {
								return 3
							}
							if line.Text == origin.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - origin.cursor.row
							if line.NewNumber > 0 && origin.line.NewNumber > 0 {
								if int(line.NewNumber-origin.line.NewNumber) < 0 {
									return -int(line.NewNumber - origin.line.NewNumber)
								}
								return int(line.NewNumber - origin.line.NewNumber)
							}
							if line.OldNumber > 0 && origin.line.OldNumber > 0 {
								if int(line.OldNumber-origin.line.OldNumber) < 0 {
									return -int(line.OldNumber - origin.line.OldNumber)
								}
								return int(line.OldNumber - origin.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() diffscreenCursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}(); ok {
			v.search.from = translated
		}
		func() {
			var query string = v.search.query
			v.search.query = query
			if query == "" {
				var cursor diffscreenDiffCursor = v.search.from
				v.cursor = cursor
				v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					for range 2 {
						height := func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}()
						if cursor.row < viewport.top {
							viewport.top = cursor.row
						}
						if cursor.row >= viewport.top+height {
							viewport.top = cursor.row - height + 1
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
					}
					return viewport
				}(v.view, v.viewport)
				v.search.miss = false
				return
			}
			cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
				if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenDiffCursor{}, false
				}
				query = strings.ToLower(query)
				y := cursor.row
				for count := 0; count < len(v.rows)-1; count++ {
					y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
					if candidate, ok := func() (diffscreenDiffCursor, bool) {
						var query string = query
						if v.rows[y].kind != diffscreenLineRow {
							if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
								var (
									y    int                = y
									pane diffscreenDiffPane = cursor.pane
								)
								for distance := 1; distance <= len(v.rows); distance++ {
									for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
										if candidate, ok := func() (diffscreenDiffCursor, bool) {
											if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
												return diffscreenDiffCursor{}, false
											}
											if candidate, ok := func() (diffscreenDiffCursor, bool) {
												cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return candidate, true
											}
											if v.split {
												cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
													if pane == diffscreenLeft {
														return diffscreenRight
													}
													return diffscreenLeft
												}()}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}
											return diffscreenDiffCursor{}, false
										}(); ok {
											return candidate, true
										}
									}
								}
								return diffscreenDiffCursor{}, false
							}
							return diffscreenDiffCursor{}, false
						}
						panes := []diffscreenDiffPane{cursor.pane}
						if v.split {
							panes = append(panes, func() diffscreenDiffPane {
								if cursor.pane == diffscreenLeft {
									return diffscreenRight
								}
								return diffscreenLeft
							}())
						}
						for _, candidatePane := range panes {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := v.rows[candidate.row]
								return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							if strings.Contains(strings.ToLower(line.Text), query) {
								return candidate, true
							}
						}
						return diffscreenDiffCursor{}, false
					}(); ok {
						return candidate, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(v.view, query, v.search.from)
			v.search.miss = !ok
			if ok {
				v.cursor = cursor
				v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					for range 2 {
						height := func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}()
						if cursor.row < viewport.top {
							viewport.top = cursor.row
						}
						if cursor.row >= viewport.top+height {
							viewport.top = cursor.row - height + 1
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
					}
					return viewport
				}(v.view, v.viewport)
			}
		}()
	}
}

// Configure changes presentation options while preserving meaningful position.
func (v *diffscreenView) Configure(options diffscreenOptions) {
	active := v.options.SideBySide && v.width >= 100
	dark := v.options.Dark
	v.options = options
	if active != (v.options.SideBySide && v.width >= 100) || dark != options.Dark {
		v.Update(v.view.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *diffscreenView) Resize(width, height int) {
	v.EndDrag()
	active := v.options.SideBySide && v.width >= 100
	v.width, v.height = width, height
	v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
		viewport.Width, viewport.Height = max(1, width), max(1, (func(height int) int { return max(1, height-3) })(height))
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view, v.viewport)
	if active != (v.options.SideBySide && v.width >= 100) {
		v.Update(v.view.patch)
	} else {
		v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) diffscreenDiffViewport {
			if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}(v.view, v.viewport, v.cursor)
	}
}

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *diffscreenView) Move(motion diffscreenMotion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case diffscreenPreviousLine:
		func() {
			next, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenDiffCursor{}, false
				}
				var pane diffscreenDiffPane = cursor.pane
				step := int(diffscreenPrivateBackward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(v.view, v.cursor)
			if !ok {
				return
			}
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current diffscreenEntry    = v.rows[selection.First.row]
							pane    diffscreenDiffPane = selection.First.pane
						)
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(next.row >= 0 && next.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[next.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || next.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[next.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = next
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.cursor = next
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[next.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || next.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if next.row < viewport.top {
						viewport.top = next.row
					}
					if next.row >= viewport.top+height {
						viewport.top = next.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}()
	case diffscreenNextLine:
		func() {
			next, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenDiffCursor{}, false
				}
				var pane diffscreenDiffPane = cursor.pane
				step := int(diffscreenPrivateForward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(v.view, v.cursor)
			if !ok {
				return
			}
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current diffscreenEntry    = v.rows[selection.First.row]
							pane    diffscreenDiffPane = selection.First.pane
						)
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(next.row >= 0 && next.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[next.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || next.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[next.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = next
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.cursor = next
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[next.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || next.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if next.row < viewport.top {
						viewport.top = next.row
					}
					if next.row >= viewport.top+height {
						viewport.top = next.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}()
	case diffscreenPreviousPage:
		func() {
			viewport, cursor := func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) (diffscreenDiffViewport, diffscreenDiffCursor) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[cursor.row]
						pane    diffscreenDiffPane = cursor.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return viewport, cursor
				}
				distance := int(diffscreenPrivateBackward) * max(1, viewport.Height/2)
				viewport.top += distance
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				target := min(len(v.rows)-1, max(0, cursor.row+distance))
				candidate, ok := func() (diffscreenDiffCursor, bool) {
					var (
						pane     diffscreenDiffPane     = cursor.pane
						viewport diffscreenDiffViewport = viewport
					)
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					for distance := 0; distance < height; distance++ {
						for _, y := range []int{target + int(diffscreenPrivateBackward)*distance, target - int(diffscreenPrivateBackward)*distance} {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								if y < viewport.top || y >= viewport.top+func() int {
									height := viewport.Height
									if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
										height--
									}
									return max(1, height)
								}() {
									return diffscreenDiffCursor{}, false
								}
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
					}
					return diffscreenDiffCursor{}, false
				}()
				if ok {
					cursor = candidate
				}
				return viewport, cursor
			}(v.view, v.viewport, v.cursor)
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current diffscreenEntry    = v.rows[selection.First.row]
							pane    diffscreenDiffPane = selection.First.pane
						)
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[cursor.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = cursor
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.viewport, v.cursor = viewport, cursor
		}()
	case diffscreenNextPage:
		func() {
			viewport, cursor := func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) (diffscreenDiffViewport, diffscreenDiffCursor) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[cursor.row]
						pane    diffscreenDiffPane = cursor.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return viewport, cursor
				}
				distance := int(diffscreenPrivateForward) * max(1, viewport.Height/2)
				viewport.top += distance
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				target := min(len(v.rows)-1, max(0, cursor.row+distance))
				candidate, ok := func() (diffscreenDiffCursor, bool) {
					var (
						pane     diffscreenDiffPane     = cursor.pane
						viewport diffscreenDiffViewport = viewport
					)
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					for distance := 0; distance < height; distance++ {
						for _, y := range []int{target + int(diffscreenPrivateForward)*distance, target - int(diffscreenPrivateForward)*distance} {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								if y < viewport.top || y >= viewport.top+func() int {
									height := viewport.Height
									if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
										height--
									}
									return max(1, height)
								}() {
									return diffscreenDiffCursor{}, false
								}
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
					}
					return diffscreenDiffCursor{}, false
				}()
				if ok {
					cursor = candidate
				}
				return viewport, cursor
			}(v.view, v.viewport, v.cursor)
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current diffscreenEntry    = v.rows[selection.First.row]
							pane    diffscreenDiffPane = selection.First.pane
						)
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[cursor.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = cursor
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.viewport, v.cursor = viewport, cursor
		}()
	case diffscreenFirstLine:
		if cursor, ok := func() (diffscreenDiffCursor, bool) {
			var v *diffscreenDiffView = v.view
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(diffscreenPrivateForward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	case diffscreenLastLine:
		if cursor, ok := func() (diffscreenDiffCursor, bool) {
			var v *diffscreenDiffView = v.view
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				var start int = len(v.rows)
				step := int(diffscreenPrivateBackward)
				for y := start + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			var start int = len(v.rows)
			step := int(diffscreenPrivateBackward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}

	default:
		func() {
			switch motion {
			case diffscreenPreviousFile:
				v.ClearSelection()
				if cursor, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffscreenDiffCursor{}, false
					}
					file, _ := func() (patchFile, bool) {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchFile{}, false
						}
						return v.patch.Files[v.rows[cursor.row].file], true
					}()
					y := cursor.row
					for {
						next, ok := func() (diffscreenDiffCursor, bool) {
							var (
								start int                = y
								pane  diffscreenDiffPane = cursor.pane
							)
							step := int(diffscreenPrivateBackward)
							for y := start + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}()
						if !ok {
							return diffscreenDiffCursor{}, false
						}
						nextFile, _ := func() (patchFile, bool) {
							if !(next.row >= 0 && next.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[next.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || next.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchFile{}, false
							}
							return v.patch.Files[v.rows[next.row].file], true
						}()
						if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
							return next, true
						}
						y = next.row
					}
				}(v.view, v.cursor); ok {
					v.cursor = cursor
					v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}
			case diffscreenNextFile:
				v.ClearSelection()
				if cursor, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffscreenDiffCursor{}, false
					}
					file, _ := func() (patchFile, bool) {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchFile{}, false
						}
						return v.patch.Files[v.rows[cursor.row].file], true
					}()
					y := cursor.row
					for {
						next, ok := func() (diffscreenDiffCursor, bool) {
							var (
								start int                = y
								pane  diffscreenDiffPane = cursor.pane
							)
							step := int(diffscreenPrivateForward)
							for y := start + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}()
						if !ok {
							return diffscreenDiffCursor{}, false
						}
						nextFile, _ := func() (patchFile, bool) {
							if !(next.row >= 0 && next.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[next.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || next.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchFile{}, false
							}
							return v.patch.Files[v.rows[next.row].file], true
						}()
						if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
							return next, true
						}
						y = next.row
					}
				}(v.view, v.cursor); ok {
					v.cursor = cursor
					v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}
			case diffscreenOldPane:
				func() {
					cursor, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffscreenDiffCursor{}, false
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var y int = cursor.row
							cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(diffscreenPrivateForward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffscreenDiffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffscreenDiffSelection{First: first, Last: first}
						selection, ok = func() (diffscreenDiffSelection, bool) {
							var (
								v         *diffscreenDiffView     = v.view
								selection diffscreenDiffSelection = selection
								cursor    diffscreenDiffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current diffscreenEntry    = v.rows[selection.First.row]
									pane    diffscreenDiffPane = selection.First.pane
								)
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			case diffscreenNewPane:
				func() {
					cursor, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffscreenDiffCursor{}, false
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var y int = cursor.row
							cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(diffscreenPrivateForward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffscreenDiffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffscreenDiffSelection{First: first, Last: first}
						selection, ok = func() (diffscreenDiffSelection, bool) {
							var (
								v         *diffscreenDiffView     = v.view
								selection diffscreenDiffSelection = selection
								cursor    diffscreenDiffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current diffscreenEntry    = v.rows[selection.First.row]
									pane    diffscreenDiffPane = selection.First.pane
								)
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			case diffscreenOtherPane:
				func() {
					var pane diffscreenDiffPane = func() diffscreenDiffPane {
						if v.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()
					cursor, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffscreenDiffCursor{}, false
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var y int = cursor.row
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(diffscreenPrivateForward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffscreenDiffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffscreenDiffCursor{}, false
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								var y int = cursor.row
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current diffscreenEntry = v.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !v.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffscreenDiffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(diffscreenPrivateForward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffscreenDiffSelection{First: first, Last: first}
						selection, ok = func() (diffscreenDiffSelection, bool) {
							var (
								v         *diffscreenDiffView     = v.view
								selection diffscreenDiffSelection = selection
								cursor    diffscreenDiffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current diffscreenEntry    = v.rows[selection.First.row]
									pane    diffscreenDiffPane = selection.First.pane
								)
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			default:
				return
			}
		}()
	}
}

// extendSelectionTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
// Align positions the focused line at the requested viewport alignment.
func (v *diffscreenView) Align(alignment diffscreenAlignment) {
	v.search.repeatMiss = false
	if _, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (patchLine, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor); !ok {
		return
	}
	v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) diffscreenDiffViewport {
		headerHeight := 0
		if viewport.Height > 1 {
			headerHeight = 1
		}
		offset := max(0, func() int {
			var height int = viewport.Height
			if diffscreenVerticalAlignment(alignment) == diffscreenMiddle {
				return height / 2
			}
			if diffscreenVerticalAlignment(alignment) == diffscreenPrivateBottom {
				return height - 1
			}
			return 0
		}()-headerHeight)
		viewport.top = cursor.row - offset
		if !(viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow) {
			viewport.top = cursor.row - func() int {
				var height int = viewport.Height
				if diffscreenVerticalAlignment(alignment) == diffscreenMiddle {
					return height / 2
				}
				if diffscreenVerticalAlignment(alignment) == diffscreenPrivateBottom {
					return height - 1
				}
				return 0
			}()
		}
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view,
		v.viewport,
		v.cursor)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *diffscreenView) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport, columns int) diffscreenDiffViewport {
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != diffscreenLineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view, v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *diffscreenView) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, rows int) (diffscreenDiffViewport, diffscreenDiffCursor) {
		rows = max(-viewport.top, min(rows, func() int {
			end := max(0, len(v.rows)-viewport.Height)
			if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
				end++
			}
			return end
		}()-viewport.top))
		viewport.top += rows
		viewport = func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current diffscreenEntry    = v.rows[cursor.row]
				pane    diffscreenDiffPane = cursor.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || func() bool {
			var viewport diffscreenDiffViewport = viewport
			return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0 && cursor.row >= viewport.top && cursor.row < viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
		}() {
			return viewport, cursor
		}
		target := max(viewport.top, min(cursor.row, viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				height--
			}
			return max(1, height)
		}()-1))
		d := diffscreenPrivateForward
		if cursor.row > target {
			d = diffscreenPrivateBackward
		}
		candidate, ok := func() (diffscreenDiffCursor, bool) {
			var (
				pane     diffscreenDiffPane         = cursor.pane
				dir      diffscreenPrivateDirection = d
				viewport diffscreenDiffViewport     = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(dir)*distance, target - int(dir)*distance} {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffscreenDiffCursor{}, false
						}
						cursor := diffscreenDiffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
			}
			return diffscreenDiffCursor{}, false
		}()
		if !ok && v.split {
			candidate, ok = func() (diffscreenDiffCursor, bool) {
				var (
					pane diffscreenDiffPane = func() diffscreenDiffPane {
						if cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()
					dir      diffscreenPrivateDirection = d
					viewport diffscreenDiffViewport     = viewport
				)
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}()
				for distance := 0; distance < height; distance++ {
					for _, y := range []int{target + int(dir)*distance, target - int(dir)*distance} {
						if cursor, ok := func() (diffscreenDiffCursor, bool) {
							if y < viewport.top || y >= viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}() {
								return diffscreenDiffCursor{}, false
							}
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return cursor, true
						}
					}
				}
				return diffscreenDiffCursor{}, false
			}()
		}
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}(v.view, v.viewport, v.cursor, rows)
	if cursor != v.cursor && !func() bool {
		if v.selection == nil {
			return true
		}
		selection, ok := func(v *diffscreenDiffView, selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := v.rows[selection.First.row]
			last := v.rows[cursor.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = cursor
			return selection, true
		}(v.view, *v.selection)
		if ok {
			v.selection = &selection
		}
		return ok
	}() {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// ToggleSelection starts a range at the focused line or clears the current
// range.
func (v *diffscreenView) ToggleSelection() {
	v.search.repeatMiss = false
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	if _, ok := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (patchLine, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor); ok {
		selection := func() diffscreenDiffSelection {
			var cursor diffscreenDiffCursor = v.cursor
			return diffscreenDiffSelection{First: cursor, Last: cursor}
		}()
		v.selection = &selection
	}
}

// ClearSelection clears the selected range and ends any drag.
func (v *diffscreenView) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

// Selected returns the focused line when no explicit range is selected.
// The bool is false when the patch has no selectable code.
func (v *diffscreenView) Selected() (patchFile, []patchLine, bool) {
	selection := v.selection
	if selection == nil {
		current := func() diffscreenDiffSelection {
			var cursor diffscreenDiffCursor = v.cursor
			return diffscreenDiffSelection{First: cursor, Last: cursor}
		}()
		selection = &current
	}
	file, ok := func() (patchFile, bool) {
		var (
			v      *diffscreenDiffView  = v.view
			cursor diffscreenDiffCursor = selection.First
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchFile{}, false
		}
		return v.patch.Files[v.rows[cursor.row].file], true
	}()
	lines := func() []patchLine {
		var (
			v         *diffscreenDiffView     = v.view
			selection diffscreenDiffSelection = *selection
		)
		if _, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[selection.Last.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || selection.Last.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := v.rows[selection.First.row]
			last := v.rows[selection.Last.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = selection.Last
			return selection, true
		}(); !ok {
			return nil
		}
		first, last := min(selection.First.row, selection.Last.row), max(selection.First.row, selection.Last.row)
		if first == last && selection.First.pane != selection.Last.pane {
			first, _ := func() (patchLine, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.First.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patchLine, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patchLine{first}
			}
			return []patchLine{first, last}
		}
		lines := make([]patchLine, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffscreenDiffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patchLine, bool) {
				var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()
	return file, lines, ok && len(lines) > 0
}

// Current returns the focused source line, independent of any selected range.
func (v *diffscreenView) Current() (patchFile, patchLine, bool) {
	file, fileOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (patchFile, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchFile{}, false
		}
		return v.patch.Files[v.rows[cursor.row].file], true
	}(v.view, v.cursor)
	line, lineOK := func(v *diffscreenDiffView, cursor diffscreenDiffCursor) (patchLine, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor)
	return file, line, fileOK && lineOK
}

// BeginSearch starts an empty search query at the current focus.
func (v *diffscreenView) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss, v.search.repeatMiss = "", false, false
	v.search.from = v.cursor
}

// InsertSearch appends text to the query and previews from the search origin.
// It starts a search session if needed; a miss retains the last successful
// focus.
func (v *diffscreenView) InsertSearch(text string) {
	if !v.search.active {
		v.BeginSearch()
	}
	func() {
		var query string = v.search.query + text
		v.search.query = query
		if query == "" {
			var cursor diffscreenDiffCursor = v.search.from
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
			v.search.miss = false
			return
		}
		cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
			if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenDiffCursor{}, false
			}
			query = strings.ToLower(query)
			y := cursor.row
			for count := 0; count < len(v.rows)-1; count++ {
				y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
					var query string = query
					if v.rows[y].kind != diffscreenLineRow {
						if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
							var (
								y    int                = y
								pane diffscreenDiffPane = cursor.pane
							)
							for distance := 1; distance <= len(v.rows); distance++ {
								for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
											return diffscreenDiffCursor{}, false
										}
										if candidate, ok := func() (diffscreenDiffCursor, bool) {
											cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current diffscreenEntry = v.rows[cursor.row]
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || cursor.pane == diffscreenRight {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}(); ok {
											return candidate, true
										}
										if v.split {
											cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
												if pane == diffscreenLeft {
													return diffscreenRight
												}
												return diffscreenLeft
											}()}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current diffscreenEntry = v.rows[cursor.row]
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || cursor.pane == diffscreenRight {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}
										return diffscreenDiffCursor{}, false
									}(); ok {
										return candidate, true
									}
								}
							}
							return diffscreenDiffCursor{}, false
						}
						return diffscreenDiffCursor{}, false
					}
					panes := []diffscreenDiffPane{cursor.pane}
					if v.split {
						panes = append(panes, func() diffscreenDiffPane {
							if cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}())
					}
					for _, candidatePane := range panes {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := v.rows[candidate.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						if strings.Contains(strings.ToLower(line.Text), query) {
							return candidate, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(); ok {
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(v.view, query, v.search.from)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	}()
}

// BackspaceSearch removes the last Unicode code point from the active query.
// An empty query restores the search origin. Outside a search it does nothing.
func (v *diffscreenView) BackspaceSearch() {
	if !v.search.active {
		return
	}
	query := []rune(v.search.query)
	if len(query) > 0 {
		query = query[:len(query)-1]
	}
	func() {
		v.search.query = string(query)
		if string(query) == "" {
			var cursor diffscreenDiffCursor = v.search.from
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
			v.search.miss = false
			return
		}
		cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
			if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenDiffCursor{}, false
			}
			query = strings.ToLower(query)
			y := cursor.row
			for count := 0; count < len(v.rows)-1; count++ {
				y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
					var query string = query
					if v.rows[y].kind != diffscreenLineRow {
						if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
							var (
								y    int                = y
								pane diffscreenDiffPane = cursor.pane
							)
							for distance := 1; distance <= len(v.rows); distance++ {
								for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
											return diffscreenDiffCursor{}, false
										}
										if candidate, ok := func() (diffscreenDiffCursor, bool) {
											cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current diffscreenEntry = v.rows[cursor.row]
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || cursor.pane == diffscreenRight {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}(); ok {
											return candidate, true
										}
										if v.split {
											cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
												if pane == diffscreenLeft {
													return diffscreenRight
												}
												return diffscreenLeft
											}()}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current diffscreenEntry = v.rows[cursor.row]
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || cursor.pane == diffscreenRight {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}
										return diffscreenDiffCursor{}, false
									}(); ok {
										return candidate, true
									}
								}
							}
							return diffscreenDiffCursor{}, false
						}
						return diffscreenDiffCursor{}, false
					}
					panes := []diffscreenDiffPane{cursor.pane}
					if v.split {
						panes = append(panes, func() diffscreenDiffPane {
							if cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}())
					}
					for _, candidatePane := range panes {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
								var current diffscreenEntry = v.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := v.rows[candidate.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						if strings.Contains(strings.ToLower(line.Text), query) {
							return candidate, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(); ok {
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(v.view, string(query), v.search.from)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != diffscreenLineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	}()
}

// AcceptSearch remembers a successful query for subsequent searches.
func (v *diffscreenView) AcceptSearch() {
	if !v.search.active {
		return
	}
	if v.search.query != "" && !v.search.miss {
		v.search.term = v.search.query
	}
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// CancelSearch restores the focus from before the active search.
func (v *diffscreenView) CancelSearch() {
	if !v.search.active {
		return
	}
	var cursor diffscreenDiffCursor = v.search.from
	v.cursor = cursor
	v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		for range 2 {
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
			if cursor.row < viewport.top {
				viewport.top = cursor.row
			}
			if cursor.row >= viewport.top+height {
				viewport.top = cursor.row - height + 1
			}
			viewport = func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
		}
		return viewport
	}(v.view, v.viewport)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// Find moves to the next remembered search match in the requested direction.
func (v *diffscreenView) Find(d diffscreenDirection) bool {
	if v.search.term == "" {
		return false
	}
	if d != diffscreenForward && d != diffscreenBackward {
		return false
	}
	cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
		if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := cursor.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateDirection(d)) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var (
							y    int                = y
							pane diffscreenDiffPane = cursor.pane
						)
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateDirection(d))*distance, y - int(diffscreenPrivateDirection(d))*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if v.split {
										cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
											if pane == diffscreenLeft {
												return diffscreenRight
											}
											return diffscreenLeft
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}
									return diffscreenDiffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffscreenDiffCursor{}, false
					}
					return diffscreenDiffCursor{}, false
				}
				panes := []diffscreenDiffPane{cursor.pane}
				if v.split {
					panes = append(panes, func() diffscreenDiffPane {
						if cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patchLine, bool) {
						if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[candidate.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchLine{}, false
						}
						current := v.rows[candidate.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffscreenDiffCursor{}, false
	}(v.view, v.search.term, v.cursor)
	v.search.repeatMiss = !ok
	if ok {
		v.cursor = cursor
		v.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
			if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}(v.view, v.viewport)
	}
	return ok
}

// Render returns the full screen, including its header, status footer, and
// final blank line. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.

// Kind selects which changes to review.

// Unstaged compares the index to the working tree and includes untracked
// files.

// Branch compares the default branch's merge base to the working tree and
// includes untracked files.

// Patch is a fully loaded snapshot. Its data belongs to the caller and is
// treated as read-only; exported fields do not enforce immutability.

// Discovered default branch, or empty if unavailable.

// File holds paths, source text, metadata, and hunks for a changed file.

// Paths are repository-relative; empty means absent on that side.

// Hunk groups diff lines under a unified diff header.

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.

// Anchor identifies the reviewed diff range and its quoted lines.

// Comment stores review feedback for a repository and diff anchor.

// Command review-my-slop provides an interactive diff review and feedback tool.

// lineRows maps source lines to display rows. Split mode pairs each deletion
// block with the immediately following additions; unmatched panes use -1.

// Package editor owns external-editor workflows and their temporary drafts,
// including Bubble Tea's terminal handoff.

// Line holds diff text and its old and new source line numbers.

// LineNumber is one-based; zero means absent on that side.

// LineKind distinguishes unchanged, added, and deleted diff lines.

// Context, Addition, and Deletion identify the kind of a diff line.

// EditComment edits a private Markdown draft and returns the resulting body.

// OpenSource opens a resolved source path at the supplied line.

// Package settings persists UI preferences in the user configuration directory.

func (v *diffscreenView) Render(err error) string {
	added, removed := func() (added, removed int) {
		for _, file := range v.view.patch.Files {
			for _, hunk := range file.Hunks {
				counts := func() [2]int {
					counts := [2]int{}
					for _, line := range hunk.Lines {
						switch line.Kind {
						case patchAddition:
							counts[0]++
						case patchDeletion:
							counts[1]++
						default:
							continue
						}
					}
					return counts
				}()
				added += counts[0]
				removed += counts[1]
			}
		}
		return added, removed
	}()
	counts := diffscreenMutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := diffscreenTitleStyle.Render("review-my-slop") + "  " + counts
	body := (func(v *diffscreenView) func() []string {
		return func() []string {
			var body []string
			if len(v.view.patch.Files) == 0 {
				empty := "No unstaged or untracked changes."
				if v.view.patch.Kind == patchBranch {
					empty = "No branch or worktree changes."
				}
				body = make([]string, (func(height int) int { return max(1, height-3) })(v.height))
				body[min(1, len(body)-1)] = diffscreenMutedStyle.Render(empty)
			} else {
				body = strings.Split(
					(func(v *diffscreenDiffView) func(
						viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
						return func(
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
							lines := make([]string, 0, viewport.Height)
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								current := v.rows[viewport.top]
								lines = append(
									lines,
									func() string {
										var path string = v.patch.Files[current.file].DisplayPath
										return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
									}(),
								)
							}
							end := min(len(v.rows), viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}())
							for y := viewport.top; y < end; y++ {
								current := v.rows[y]
								if v.split && current.kind == diffscreenLineRow {
									lines = append(
										lines,
										(func(v *diffscreenDiffView) func(
											current diffscreenEntry, y int,
											viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											return func(
												current diffscreenEntry, y int,
												viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
												leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
													inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
													return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
												}()
												inlineLocalLeft1 := func() string {
													index := func() int {
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || diffscreenLeft == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}()
													if index < 0 {
														return strings.Repeat(" ", leftWidth)
													}
													line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
													text := current.right
													numberValue := line.NewNumber
													if diffscreenLeft == diffscreenLeft {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patchAddition {
														prefix = diffscreenAddedStyle.Render("+") + " "
													}
													if line.Kind == patchDeletion {
														prefix = diffscreenRemovedStyle.Render("-") + " "
													}
													gutter := fmt.Sprintf("%5s ", func() string {
														if numberValue == 0 {
															return ""
														}
														return strconv.Itoa(int(numberValue))
													}())
													value := gutter + func() string {
														var (
															value string = prefix + text
															width int    = leftWidth - lipgloss.Width(gutter)
														)
														if width <= 0 {
															return ""
														}
														value = strings.ReplaceAll(value, "\t", "    ")
														if viewport.LeftColumn > 0 {
															value = ansi.TruncateLeft(value, viewport.LeftColumn, "")
														}
														value = ansi.Truncate(value, width, "")
														if padding := width - lipgloss.Width(value); padding > 0 {
															value += strings.Repeat(" ", padding)
														}
														return value
													}()
													style := func() lipgloss.Style {
														lightDark := lipgloss.LightDark(v.dark)
														switch line.Kind {
														case patchAddition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patchDeletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return diffscreenContextStyle
														}
													}()
													colors := diffscreenPreserveForeground
													candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
													if func() bool {
														if selection == nil {
															return false
														}
														first, last := selection.First, selection.Last
														if first.row == last.row && first.pane != last.pane {
															return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
														}
														if first.pane != candidate.pane {
															return false
														}
														return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
													}() {
														style, colors = func() lipgloss.Style {
															var dark bool = v.dark
															return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
														}(), diffscreenDiscardForeground
													}
													if cursor == candidate {
														style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
													}
													{
														var (
															style  lipgloss.Style             = style
															value  string                     = value
															colors diffscreenForegroundPolicy = colors
														)
														value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
															parameters := sequence[2 : len(sequence)-1]
															if parameters == "" {
																return sequence
															}
															parts := strings.Split(parameters, ";")
															filtered := func() []string {
																filtered := make([]string, 0, len(parts))
																for index := 0; index < len(parts); index++ {
																	code, err := strconv.Atoi(parts[index])
																	if err != nil {
																		filtered = append(filtered, parts[index])
																		continue
																	}
																	if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
																		index = func() int {
																			if index+1 >= len(parts) {
																				return index
																			}
																			switch parts[index+1] {
																			case "2":
																				return min(index+4, len(parts)-1)
																			case "5":
																				return min(index+2, len(parts)-1)
																			default:
																				return index
																			}
																		}()
																		continue
																	}
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																		continue
																	}
																	filtered = append(filtered, parts[index])
																}
																return filtered
															}()
															if len(filtered) == 0 {
																return ""
															}
															return "\x1b[" + strings.Join(filtered, ";") + "m"
														})
														fitted := func() string {
															var value string = value
															if leftWidth <= 0 {
																return ""
															}
															value = strings.ReplaceAll(value, "\t", "    ")
															if 0 > 0 {
																value = ansi.TruncateLeft(value, 0, "")
															}
															value = ansi.Truncate(value, leftWidth, "")
															if padding := leftWidth - lipgloss.Width(value); padding > 0 {
																value += strings.Repeat(" ", padding)
															}
															return value
														}()
														prefix := func() string {
															const marker = "\x00"
															rendered := style.Render(marker)
															index := strings.Index(rendered, marker)
															if index < 0 {
																return ""
															}
															return rendered[:index]
														}()
														if prefix != "" {
															fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
															fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
														}
														return style.Render(fitted)
													}
												}()
												inlineLocalRight1 := (func(v *diffscreenDiffView) func(
													current diffscreenEntry, y int,
													pane diffscreenDiffPane, width, offset int,
													cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
													return func(
														current diffscreenEntry, y int,
														pane diffscreenDiffPane, width, offset int,
														cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
														index := func() int {
															if current.kind != diffscreenLineRow {
																return -1
															}
															if !v.split || pane == diffscreenRight {
																return current.rightLine
															}
															return current.leftLine
														}()
														if index < 0 {
															return strings.Repeat(" ", width)
														}
														line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
														text := current.right
														numberValue := line.NewNumber
														if pane == diffscreenLeft {
															text, numberValue = current.left, line.OldNumber
														}
														prefix := "  "
														if line.Kind == patchAddition {
															prefix = diffscreenAddedStyle.Render("+") + " "
														}
														if line.Kind == patchDeletion {
															prefix = diffscreenRemovedStyle.Render("-") + " "
														}
														gutter := fmt.Sprintf("%5s ", func() string {
															if numberValue == 0 {
																return ""
															}
															return strconv.Itoa(int(numberValue))
														}())
														value := gutter + func() string {
															var (
																value string = prefix + text
																width int    = width - lipgloss.Width(gutter)
															)
															if width <= 0 {
																return ""
															}
															value = strings.ReplaceAll(value, "\t", "    ")
															if offset > 0 {
																value = ansi.TruncateLeft(value, offset, "")
															}
															value = ansi.Truncate(value, width, "")
															if padding := width - lipgloss.Width(value); padding > 0 {
																value += strings.Repeat(" ", padding)
															}
															return value
														}()
														style := func() lipgloss.Style {
															lightDark := lipgloss.LightDark(v.dark)
															switch line.Kind {
															case patchAddition:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
															case patchDeletion:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
															default:
																return diffscreenContextStyle
															}
														}()
														colors := diffscreenPreserveForeground
														candidate := diffscreenDiffCursor{row: y, pane: pane}
														if func() bool {
															if selection == nil {
																return false
															}
															first, last := selection.First, selection.Last
															if first.row == last.row && first.pane != last.pane {
																return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
															}
															if first.pane != candidate.pane {
																return false
															}
															return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
														}() {
															style, colors = func() lipgloss.Style {
																var dark bool = v.dark
																return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
															}(), diffscreenDiscardForeground
														}
														if cursor == candidate {
															style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
														}
														{
															var (
																style  lipgloss.Style             = style
																value  string                     = value
																colors diffscreenForegroundPolicy = colors
															)
															value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
																parameters := sequence[2 : len(sequence)-1]
																if parameters == "" {
																	return sequence
																}
																parts := strings.Split(parameters, ";")
																filtered := func() []string {
																	filtered := make([]string, 0, len(parts))
																	for index := 0; index < len(parts); index++ {
																		code, err := strconv.Atoi(parts[index])
																		if err != nil {
																			filtered = append(filtered, parts[index])
																			continue
																		}
																		if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
																			index = func() int {
																				if index+1 >= len(parts) {
																					return index
																				}
																				switch parts[index+1] {
																				case "2":
																					return min(index+4, len(parts)-1)
																				case "5":
																					return min(index+2, len(parts)-1)
																				default:
																					return index
																				}
																			}()
																			continue
																		}
																		if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																			continue
																		}
																		filtered = append(filtered, parts[index])
																	}
																	return filtered
																}()
																if len(filtered) == 0 {
																	return ""
																}
																return "\x1b[" + strings.Join(filtered, ";") + "m"
															})
															fitted := func() string {
																var value string = value
																if width <= 0 {
																	return ""
																}
																value = strings.ReplaceAll(value, "\t", "    ")
																if 0 > 0 {
																	value = ansi.TruncateLeft(value, 0, "")
																}
																value = ansi.Truncate(value, width, "")
																if padding := width - lipgloss.Width(value); padding > 0 {
																	value += strings.Repeat(" ", padding)
																}
																return value
															}()
															prefix := func() string {
																const marker = "\x00"
																rendered := style.Render(marker)
																index := strings.Index(rendered, marker)
																if index < 0 {
																	return ""
																}
																return rendered[:index]
															}()
															if prefix != "" {
																fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
																fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
															}
															return style.Render(fitted)
														}
													}
												}(v))(
													current,
													y,
													diffscreenRight,
													rightWidth,
													viewport.LeftColumn,
													cursor,
													selection,
												)
												return inlineLocalLeft1 + " │ " + inlineLocalRight1
											}
										}(v))(current, y, viewport, cursor, selection),
									)
								} else {
									lines = append(
										lines,
										func() string {
											var y int = y
											width := max(20, viewport.Width)
											switch current.kind {
											case diffscreenFileRow:
												return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
											case diffscreenMetadataRow:
												return diffscreenMetadataStyle.Render("  " + current.text)
											case diffscreenHunkRow:
												return diffscreenHunkStyle.Render(current.text)
											case diffscreenLineRow:
												hunk := v.patch.Files[current.file].Hunks[current.hunk]
												line := hunk.Lines[current.rightLine]
												prefix := " "
												if line.Kind == patchAddition {
													prefix = diffscreenAddedStyle.Render("+")
												}
												if line.Kind == patchDeletion {
													prefix = diffscreenRemovedStyle.Render("-")
												}
												gutter := fmt.Sprintf("%5s %5s %s ", func() string {
													if line.OldNumber == 0 {
														return ""
													}
													return strconv.Itoa(int(line.OldNumber))
												}(), func() string {
													if line.NewNumber == 0 {
														return ""
													}
													return strconv.Itoa(int(line.NewNumber))
												}(), prefix)
												value := gutter + func() string {
													var (
														value string = current.text
														width int    = width - lipgloss.Width(gutter)
													)
													if width <= 0 {
														return ""
													}
													value = strings.ReplaceAll(value, "\t", "    ")
													if viewport.LeftColumn > 0 {
														value = ansi.TruncateLeft(value, viewport.LeftColumn, "")
													}
													value = ansi.Truncate(value, width, "")
													if padding := width - lipgloss.Width(value); padding > 0 {
														value += strings.Repeat(" ", padding)
													}
													return value
												}()
												style := func() lipgloss.Style {
													lightDark := lipgloss.LightDark(v.dark)
													switch line.Kind {
													case patchAddition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patchDeletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return diffscreenContextStyle
													}
												}()
												colors := diffscreenPreserveForeground
												candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
												if func() bool {
													if selection == nil {
														return false
													}
													first, last := selection.First, selection.Last
													if first.row == last.row && first.pane != last.pane {
														return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
													}
													if first.pane != candidate.pane {
														return false
													}
													return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
												}() {
													style, colors = func() lipgloss.Style {
														var dark bool = v.dark
														return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
													}(), diffscreenDiscardForeground
												}
												if cursor.row == y {
													style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
												}
												{
													var (
														style  lipgloss.Style             = style
														value  string                     = value
														colors diffscreenForegroundPolicy = colors
													)
													value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
														parameters := sequence[2 : len(sequence)-1]
														if parameters == "" {
															return sequence
														}
														parts := strings.Split(parameters, ";")
														filtered := func() []string {
															filtered := make([]string, 0, len(parts))
															for index := 0; index < len(parts); index++ {
																code, err := strconv.Atoi(parts[index])
																if err != nil {
																	filtered = append(filtered, parts[index])
																	continue
																}
																if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
																	index = func() int {
																		if index+1 >= len(parts) {
																			return index
																		}
																		switch parts[index+1] {
																		case "2":
																			return min(index+4, len(parts)-1)
																		case "5":
																			return min(index+2, len(parts)-1)
																		default:
																			return index
																		}
																	}()
																	continue
																}
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																	continue
																}
																filtered = append(filtered, parts[index])
															}
															return filtered
														}()
														if len(filtered) == 0 {
															return ""
														}
														return "\x1b[" + strings.Join(filtered, ";") + "m"
													})
													fitted := func() string {
														var value string = value
														if width <= 0 {
															return ""
														}
														value = strings.ReplaceAll(value, "\t", "    ")
														if 0 > 0 {
															value = ansi.TruncateLeft(value, 0, "")
														}
														value = ansi.Truncate(value, width, "")
														if padding := width - lipgloss.Width(value); padding > 0 {
															value += strings.Repeat(" ", padding)
														}
														return value
													}()
													prefix := func() string {
														const marker = "\x00"
														rendered := style.Render(marker)
														index := strings.Index(rendered, marker)
														if index < 0 {
															return ""
														}
														return rendered[:index]
													}()
													if prefix != "" {
														fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
														fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
													}
													return style.Render(fitted)
												}
											}
											return ""
										}(),
									)
								}
							}
							for len(lines) < viewport.Height {
								lines = append(lines, "")
							}
							return strings.Join(lines, "\n")
						}
					}(v.view))(v.viewport, v.cursor, v.selection),
					"\n",
				)
			}
			return body
		}
	}(v))()
	status := func() string {
		status := "j/k/h/l move  c comment  ? help  q quit"
		if v.search.active {
			status = "/" + v.search.query + diffscreenCursorStyle.Render(" ")
			if v.search.miss {
				status += diffscreenErrorStyle.Render("  no matches")
			}
		} else if v.selection != nil {
			status = "visual selection  j/k extend  c comment  Esc cancel"
		}
		return status
	}()
	footer := diffscreenMutedStyle.Render(status)
	if v.search.repeatMiss {
		message := fmt.Sprintf("no matches for %q", v.search.term)
		footer = diffscreenErrorStyle.Render(message)
	}
	if err != nil {
		footer = diffscreenErrorStyle.Render(strings.ReplaceAll(err.Error(), "\n", " "))
	}
	return (func(header string, body []string, footer string, height int) string {
		bodyHeight := (func(height int) int { return max(1, height-3) })(height)
		lines := make([]string,

			// Add validates and stores a comment, assigning an ID and timestamp when
			// absent.
			0, bodyHeight+3)
		lines = append(lines, header)
		lines = append(lines, body[:min(len(body), bodyHeight)]...)
		for len(lines) < bodyHeight+1 {
			lines = append(lines, "")
		}
		lines = append(lines, footer, "")
		return strings.Join(lines, "\n")
	})(header, body, func() string {
		var value string = footer
		label := "local changes"
		if v.view.patch.Kind == patchBranch {
			label = "branch changes from " + v.view.patch.Branch
		}
		if v.viewport.top > 0 {
			label += fmt.Sprintf(" (%d%%)", func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
				if len(v.rows) == 0 {
					return 0
				}
				inlineLocalBottom1 := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				return inlineLocalBottom1 * 100 / len(v.rows)
			}(v.view, v.viewport))
		}
		inlineLocalRight1 := diffscreenMutedStyle.Render(label)
		width := max(20, v.width)
		value = ansi.Truncate(value, max(0, width-lipgloss.Width(inlineLocalRight1)-1), "")
		return value + strings.Repeat(" ", max(1, width-lipgloss.Width(value)-lipgloss.Width(inlineLocalRight1))) + inlineLocalRight1
	}(), v.height)
}

var (
	diffscreenTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	diffscreenMutedStyle = lipgloss.NewStyle().Faint(true)
	diffscreenErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)

type patchKind uint8

const (
	patchUnstaged patchKind = iota

	patchBranch
)

type patchPatch struct {
	Root   string
	Kind   patchKind
	Branch string
	Files  []patchFile
}

type patchFile struct {
	OldPath     string
	NewPath     string
	DisplayPath string
	OldSource   string
	NewSource   string
	Metadata    []string
	Hunks       []patchHunk
}

type patchHunk struct {
	Header string
	Lines  []patchLine
}
type diffscreenForegroundPolicy uint8

const (
	diffscreenPreserveForeground diffscreenForegroundPolicy = iota
	diffscreenDiscardForeground
)

var (
	diffscreenAnsiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	diffscreenFileStyle      = lipgloss.NewStyle().Bold(true)
	diffscreenMetadataStyle  = lipgloss.NewStyle().Faint(true)
	diffscreenHunkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	diffscreenContextStyle   = lipgloss.NewStyle()
	diffscreenAddedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Green)
	diffscreenRemovedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	diffscreenCursorStyle    = lipgloss.NewStyle().Reverse(true)
)

type diffscreenCursorMatch struct {
	cursor         diffscreenDiffCursor
	rank, distance int
}
type diffscreenDiffPane uint8

const (
	diffscreenLeft diffscreenDiffPane = iota
	diffscreenRight
)

type diffscreenDiffCursor struct {
	row  int
	pane diffscreenDiffPane
}

type diffscreenDiffViewport struct {
	top        int
	LeftColumn int
	Width      int
	Height     int
}

type diffscreenDiffSelection struct {
	First diffscreenDiffCursor
	Last  diffscreenDiffCursor
}

type diffscreenPrivateDirection int8

const (
	diffscreenPrivateBackward diffscreenPrivateDirection = -1
	diffscreenPrivateForward  diffscreenPrivateDirection = 1
)

type diffscreenVerticalAlignment uint8

const (
	diffscreenPrivateTop diffscreenVerticalAlignment = iota
	diffscreenMiddle
	diffscreenPrivateBottom
)

type commentsAnchor struct {
	FilePath    string   `json:"file"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type commentsComment struct {
	ID         string         `json:"id"`
	Repository string         `json:"repository"`
	CreatedAt  time.Time      `json:"created_at"`
	Anchor     commentsAnchor `json:"anchor"`
	Body       string         `json:"body"`
}

const mainErrorExitCode = 1

func main() {
	if err := (func(ctx context.Context, args []string, output io.Writer) error {
		if len(args) == 0 {
			return (func(ctx context.Context) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
					directory, err := os.Getwd()
					if err != nil {
						return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
					}
					{
						if kind != patchUnstaged && kind != patchBranch {
							return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
						}
						if err := ctx.Err(); err != nil {
							return patchPatch{}, err
						}
						root, err := func() (string, error) {
							rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
							if err != nil {
								return "", err
							}
							root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
							if err != nil {
								return "", fmt.Errorf("resolve repository root: %w", err)
							}
							return root, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						branch, err := func() (string, error) {
							out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
							if ctx.Err() != nil {
								return "", ctx.Err()
							}
							if err == nil {
								return strings.TrimSpace(string(out)), nil
							}
							for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
								_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
								if ctx.Err() != nil {
									return "", ctx.Err()
								}
								if err == nil {
									return candidate, nil
								}
							}
							return "", nil
						}()
						if err != nil {
							return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
						}
						revisions, readOld, err := func() ([]string, patchSourceReader, error) {
							if kind != patchBranch {
								return nil, func(ctx context.Context, run patchRunner, root, path string) string {
									if path == "" || path == "/dev/null" {
										return ""
									}
									out, err := run.Run(ctx, root, "show", ":"+path)
									if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}, nil
							}
							if branch == "" {
								return nil, nil, errors.New("default branch is unavailable")
							}
							baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
							if err != nil {
								return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
							}
							base := strings.TrimSpace(string(baseBytes))
							readOld := func(ctx context.Context, run patchRunner, root, path string) string {
								if path == "" || path == "/dev/null" {
									return ""
								}
								out, err := run.Run(ctx, root, "show", base+":"+path)
								if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}
							return []string{base}, readOld, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						raw, err := func() ([]byte, error) {
							args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
							args = append(args, revisions...)
							args = append(args, "--")
							return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
						}()
						if err != nil {
							return patchPatch{}, err
						}
						{
							files, err := func() ([]patchFile, error) {
								if len(bytes.TrimSpace(raw)) == 0 {
									return nil, nil
								}
								parsed, err := diff.ParseMultiFileDiff(raw)
								if err != nil {
									return nil, fmt.Errorf("parse git diff: %w", err)
								}
								files := make([]patchFile, 0, len(parsed))
								for _, fd := range parsed {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, err := func() (patchFile, error) {
										oldPath := func() string {
											var path string = fd.OrigName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										newPath := func() string {
											var path string = fd.NewName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										display := newPath
										if display == "" || display == "/dev/null" {
											display = oldPath
										}
										file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
											var value string = display
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}(), Metadata: func() []string {
											var values []string = fd.Extended
											result := make([]string, len(values))
											for index, value := range values {
												result[index] = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}
											return result
										}()}
										file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
										file.NewSource = func() string {
											if newPath == "" || newPath == "/dev/null" {
												return ""
											}
											full := filepath.Join(root, filepath.FromSlash(newPath))
											info, err := os.Lstat(full)
											if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
												return ""
											}
											out, err := os.ReadFile(full)
											if err != nil || bytes.IndexByte(out, 0) >= 0 {
												return ""
											}
											var result strings.Builder
											for _, r := range string(out) {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}()
										for _, h := range fd.Hunks {
											lines, err := func() ([]patchLine, error) {
												var (
													oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
													body             []byte = h.Body
												)
												rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
												if len(body) == 0 {
													return []patchLine{}, nil
												}
												lines := make([]patchLine, 0, len(rawLines))
												for _, raw := range rawLines {
													if len(raw) == 0 {
														return nil, errors.New("malformed empty diff line")
													}
													text := func() string {
														var value string = string(raw[1:])
														return strings.ReplaceAll(func() string {
															var result strings.Builder
															for _, r := range value {
																switch {
																case r == '\n' || r == '\t':
																	_, _ = result.WriteRune(r)
																case r == '\r':
																	_, _ = result.WriteString(`\r`)
																case r < 0x20 || r == 0x7f:
																	fmt.Fprintf(&result, `\x%02x`, r)
																default:
																	_, _ = result.WriteRune(r)
																}
															}
															return result.String()
														}(), "\n", `\n`)
													}()
													switch raw[0] {
													case ' ':
														lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
														oldLine++
														newLine++
													case '+':
														lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
														newLine++
													case '-':
														lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
														oldLine++
													case '\\':
													default:
														return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
													}
												}
												return lines, nil
											}()
											if err != nil {
												return patchFile{}, fmt.Errorf("%s: %w", display, err)
											}
											file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
												header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
												if h.Section != "" {
													header += " " + h.Section
												}
												return header
											}(), Lines: lines})
										}
										return file, nil
									}()
									if err != nil {
										return nil, err
									}
									files = append(files, file)
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							untracked, err := func() ([]patchFile, error) {
								out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
								if err != nil {
									return nil, err
								}
								var files []patchFile
								for _, path := range func() []string {
									var paths []string
									for path := range bytes.SplitSeq(out, []byte{0}) {
										if len(path) > 0 {
											paths = append(paths, string(path))
										}
									}
									return paths
								}() {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, include, err := func() (patchFile, bool, error) {
										full := filepath.Join(root, filepath.FromSlash(path))
										info, err := os.Lstat(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
										}
										if info.Mode()&os.ModeSymlink != 0 {
											target, err := os.Readlink(full)
											if err != nil {
												return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
											}
											return func() patchFile {
												var content string = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range target {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
												sourceLines := func() []string {
													var content string = content
													content = strings.TrimSuffix(content, "\n")
													if content == "" {
														return nil
													}
													return strings.Split(content, "\n")
												}()
												lines := make([]patchLine, 0, len(sourceLines))
												for i, line := range sourceLines {
													lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
												}
												return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range path {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
											}(), true, nil
										}
										if !info.Mode().IsRegular() {
											return patchFile{}, false, nil
										}
										if info.Size() > patchMaxFileBytes {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
										}
										content, err := os.ReadFile(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
										}
										if bytes.IndexByte(content, 0) >= 0 {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
										}
										return func() patchFile {
											var content string = func() string {
												var result strings.Builder
												for _, r := range string(content) {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}()
											sourceLines := func() []string {
												var content string = content
												content = strings.TrimSuffix(content, "\n")
												if content == "" {
													return nil
												}
												return strings.Split(content, "\n")
											}()
											lines := make([]patchLine, 0, len(sourceLines))
											for i, line := range sourceLines {
												lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
											}
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
										}(), true, nil
									}()
									if err != nil {
										return nil, err
									}
									if include {
										files = append(files, file)
									}
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							files = append(files, untracked...)
							slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
							if err := ctx.Err(); err != nil {
								return patchPatch{}, err
							}
							return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
						}
					}
				})(ctx, patchUnstaged)
				if err != nil {
					return err
				}
				return (func(
					ctx context.Context,
					currentPatch patchPatch,
					inlineLocalCommentStore1 commentsStore,
					refresh func(context.Context, patchKind) (patchPatch, error),
				) error {
					return (func(
						ctx context.Context,
						currentPatch patchPatch,
						store appCommentStore, refresh func(context.Context, patchKind) (patchPatch, error),
						terminalSize func() appSize, launch func(appModel, ...tea.ProgramOption) error,
					) error {
						pending, err := store.List(currentPatch.Root)
						if err != nil {
							return err
						}
						dimensions := terminalSize()
						m, err := func() (appModel, error) {
							preferences, err := (func() (settingsPreferences, error) {
								path, err := (func() (string, error) {
									config, err := os.UserConfigDir()
									if err != nil {
										return "", fmt.Errorf("resolve UI settings directory: %w", err)
									}
									return filepath.Join(config, "review-my-slop", "ui.json"), nil
								})()
								if err != nil {
									return settingsPreferences{}, err
								}
								return (func(path string) (settingsPreferences, error) {
									data, err := os.ReadFile(path)
									if errors.Is(err, os.ErrNotExist) {
										return settingsPreferences{}, nil
									}
									if err != nil {
										return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
									}
									var settings settingsPreferences
									if err := json.Unmarshal(data, &settings); err != nil {
										return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
									}
									return settings, nil
								})(path)
							})()
							if err != nil {
								return appModel{}, err
							}
							m := func() appModel {
								var layout appInitialLayout = appInitialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error {
									return (func(preferences settingsPreferences) error {
										path, err := (func() (string, error) {
											config, err := os.UserConfigDir()
											if err != nil {
												return "", fmt.Errorf("resolve UI settings directory: %w", err)
											}
											return filepath.Join(config, "review-my-slop", "ui.json"), nil
										})()
										if err != nil {
											return err
										}
										return (func(path string, preferences settingsPreferences) error {
											if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
												return fmt.Errorf("create UI settings directory: %w", err)
											}
											data, err := json.Marshal(preferences)
											if err != nil {
												return fmt.Errorf("encode UI settings: %w", err)
											}
											data = append(data, '\n')
											temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
											if err != nil {
												return fmt.Errorf("create UI settings file: %w", err)
											}
											temporaryPath := temporary.Name()
											defer os.Remove(temporaryPath)
											if err := temporary.Chmod(0o600); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("secure UI settings file: %w", err)
											}
											if _, err := temporary.Write(data); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("write UI settings: %w", err)
											}
											if err := temporary.Close(); err != nil {
												return fmt.Errorf("close UI settings: %w", err)
											}
											if err := os.Rename(temporaryPath, path); err != nil {
												return fmt.Errorf("replace UI settings: %w", err)
											}
											return nil
										})(path, preferences)
									})(settingsPreferences{SideBySide: enabled})
								}, size: dimensions}
								inlineLocalSize1 := layout.size
								if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
									inlineLocalSize1 = appDefaultSize
								}
								m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
									v := &commentscreenView{width: 80, height: 30}
									v.Update(items)
									return v
								})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
									comment.Repository = current.Root
									if comment.ID != "" {
										if err := store.Update(comment); err != nil {
											return commentsComment{}, err
										}
										return comment, nil
									}
									return store.Add(comment)
								}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
									m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
									for _, sequence := range sequences {
										sequence.Keys = append([]string(nil), sequence.Keys...)
										m.sequences[sequence.Prefix] = sequence
									}
									return m
								})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
								m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
									v := &diffscreenView{options: options, width: 80, height: 30}
									v.view = func() *diffscreenDiffView {
										{
											var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
											v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
											for fileIndex, file := range p.Files {
												var file patchFile = file
												v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
												for _, metadata := range file.Metadata {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
												}
												highlighted := func() diffscreenPair {
													var file *patchFile = &file
													theme := diffscreenLightTheme
													if v.dark {
														theme = diffscreenDarkTheme
													}
													return diffscreenPair{Old: func() []string {
														var filename, source, theme string = file.OldPath, file.OldSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}(), New: func() []string {
														var filename, source, theme string = file.NewPath, file.NewSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}()}
												}()
												for hunkIndex, hunk := range file.Hunks {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
														if strings.HasPrefix(hunk.Header, "@@") {
															return hunk.Header
														}
														return "@@ " + hunk.Header
													}()})
													for _, indices := range func() [][2]int {
														rows := make([][2]int, 0, len(hunk.Lines))
														for index := 0; index < len(hunk.Lines); {
															if !v.split || hunk.Lines[index].Kind == patchContext {
																rows = append(rows, [2]int{index, index})
																index++
																continue
															}
															if hunk.Lines[index].Kind == patchAddition {
																rows = append(rows, [2]int{-1, index})
																index++
																continue
															}
															addedStart := func() int {
																var start int = index
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
																	start++
																}
																return start
															}()
															end := func() int {
																var start int = addedStart
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
																	start++
																}
																return start
															}()
															rows = append(rows, func() [][2]int {
																count := max(addedStart-index, end-addedStart)
																rows := make([][2]int, 0, count)
																for offset := 0; offset < count; offset++ {
																	inlineLocalPair2 := [2]int{-1, -1}
																	if index+offset < addedStart {
																		inlineLocalPair2[0] = index + offset
																	}
																	if addedStart+offset < end {
																		inlineLocalPair2[1] = addedStart + offset
																	}
																	rows = append(rows, inlineLocalPair2)
																}
																return rows
															}()...)
															index = end
														}
														return rows
													}() {
														row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
														func() {
															var row *diffscreenEntry = &row
															if !v.split {
																row.text = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
																return
															}
															if row.leftLine >= 0 {
																row.left = func() string {
																	var line patchLine = hunk.Lines[row.leftLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
															if row.rightLine >= 0 {
																row.right = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
														}()
														v.rows = append(v.rows, row)
													}
												}
											}
											return v
										}
									}()
									v.viewport = func(v *diffscreenDiffView, width, height int) diffscreenDiffViewport {
										var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
										viewport.Width, viewport.Height = max(1, width), max(1, height)
										{
											var viewport diffscreenDiffViewport = viewport
											viewport.top = max(0, min(viewport.top, func() int {
												end := max(0, len(v.rows)-viewport.Height)
												if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
													end++
												}
												return end
											}()))
											viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
												contentWidth := max(1, viewport.Width-14)
												extra := 0
												if v.split {
													contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
												}
												longest := 0
												for _, current := range v.rows {
													if current.kind != diffscreenLineRow {
														continue
													}
													longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
												}
												return max(0, longest-contentWidth)
											}()))
											return viewport
										}
									}(v.view, v.width, (func(height int) int { return max(1, height-3) })(v.height))
									v.cursor, _ = func() (diffscreenDiffCursor, bool) {
										var v *diffscreenDiffView = v.view
										if cursor, ok := func() (diffscreenDiffCursor, bool) {
											step := int(diffscreenPrivateForward)
											for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
												if cursor, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return cursor, true
												}
											}
											return diffscreenDiffCursor{}, false
										}(); ok {
											return cursor, true
										}
										step := int(diffscreenPrivateForward)
										for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
											if cursor, ok := func() (diffscreenDiffCursor, bool) {
												cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return cursor, true
											}
										}
										return diffscreenDiffCursor{}, false
									}()
									return v
								})(currentPatch, m.diffOptions)
								m.diffView.Resize(m.width, m.height)
								m.commentView.Resize(m.width, m.height)
								return m
							}()
							m.delete = func(comment commentsComment, current patchPatch) error {
								return store.Delete(current.Root, comment.ID)
							}
							m.load = func() ([]commentsComment, error) { return store.List(currentPatch.Root) }
							return m, nil
						}()
						if err != nil {
							return err
						}
						m.ctx = ctx
						if refresh != nil {
							m.refresh = func(kind patchKind) (patchPatch, error) { return refresh(ctx, kind) }
						}
						return launch(
							m,
							tea.WithContext(ctx),
							tea.WithWindowSize(dimensions.Width, dimensions.Height),
						)
					})(
						ctx,
						currentPatch,
						inlineLocalCommentStore1,
						refresh,
						(func() appSize {
							if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							return appDefaultSize
						}),
						func(
							m appModel, options ...tea.ProgramOption,
						) error {
							_, err := tea.NewProgram(m, options...).Run()
							return err
						},
					)
				})(ctx, currentPatch, commentStore, (func(ctx context.Context, kind patchKind) (patchPatch, error) {
					directory, err := os.Getwd()
					if err != nil {
						return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
					}
					{
						if kind != patchUnstaged && kind != patchBranch {
							return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
						}
						if err := ctx.Err(); err != nil {
							return patchPatch{}, err
						}
						root, err := func() (string, error) {
							rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
							if err != nil {
								return "", err
							}
							root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
							if err != nil {
								return "", fmt.Errorf("resolve repository root: %w", err)
							}
							return root, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						branch, err := func() (string, error) {
							out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
							if ctx.Err() != nil {
								return "", ctx.Err()
							}
							if err == nil {
								return strings.TrimSpace(string(out)), nil
							}
							for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
								_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
								if ctx.Err() != nil {
									return "", ctx.Err()
								}
								if err == nil {
									return candidate, nil
								}
							}
							return "", nil
						}()
						if err != nil {
							return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
						}
						revisions, readOld, err := func() ([]string, patchSourceReader, error) {
							if kind != patchBranch {
								return nil, func(ctx context.Context, run patchRunner, root, path string) string {
									if path == "" || path == "/dev/null" {
										return ""
									}
									out, err := run.Run(ctx, root, "show", ":"+path)
									if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}, nil
							}
							if branch == "" {
								return nil, nil, errors.New("default branch is unavailable")
							}
							baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
							if err != nil {
								return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
							}
							base := strings.TrimSpace(string(baseBytes))
							readOld := func(ctx context.Context, run patchRunner, root, path string) string {
								if path == "" || path == "/dev/null" {
									return ""
								}
								out, err := run.Run(ctx, root, "show", base+":"+path)
								if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}
							return []string{base}, readOld, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						raw, err := func() ([]byte, error) {
							args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
							args = append(args, revisions...)
							args = append(args, "--")
							return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
						}()
						if err != nil {
							return patchPatch{}, err
						}
						{
							files, err := func() ([]patchFile, error) {
								if len(bytes.TrimSpace(raw)) == 0 {
									return nil, nil
								}
								parsed, err := diff.ParseMultiFileDiff(raw)
								if err != nil {
									return nil, fmt.Errorf("parse git diff: %w", err)
								}
								files := make([]patchFile, 0, len(parsed))
								for _, fd := range parsed {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, err := func() (patchFile, error) {
										oldPath := func() string {
											var path string = fd.OrigName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										newPath := func() string {
											var path string = fd.NewName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										display := newPath
										if display == "" || display == "/dev/null" {
											display = oldPath
										}
										file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
											var value string = display
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}(), Metadata: func() []string {
											var values []string = fd.Extended
											result := make([]string, len(values))
											for index, value := range values {
												result[index] = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}
											return result
										}()}
										file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
										file.NewSource = func() string {
											if newPath == "" || newPath == "/dev/null" {
												return ""
											}
											full := filepath.Join(root, filepath.FromSlash(newPath))
											info, err := os.Lstat(full)
											if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
												return ""
											}
											out, err := os.ReadFile(full)
											if err != nil || bytes.IndexByte(out, 0) >= 0 {
												return ""
											}
											var result strings.Builder
											for _, r := range string(out) {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}()
										for _, h := range fd.Hunks {
											lines, err := func() ([]patchLine, error) {
												var (
													oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
													body             []byte = h.Body
												)
												rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
												if len(body) == 0 {
													return []patchLine{}, nil
												}
												lines := make([]patchLine, 0, len(rawLines))
												for _, raw := range rawLines {
													if len(raw) == 0 {
														return nil, errors.New("malformed empty diff line")
													}
													text := func() string {
														var value string = string(raw[1:])
														return strings.ReplaceAll(func() string {
															var result strings.Builder
															for _, r := range value {
																switch {
																case r == '\n' || r == '\t':
																	_, _ = result.WriteRune(r)
																case r == '\r':
																	_, _ = result.WriteString(`\r`)
																case r < 0x20 || r == 0x7f:
																	fmt.Fprintf(&result, `\x%02x`, r)
																default:
																	_, _ = result.WriteRune(r)
																}
															}
															return result.String()
														}(), "\n", `\n`)
													}()
													switch raw[0] {
													case ' ':
														lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
														oldLine++
														newLine++
													case '+':
														lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
														newLine++
													case '-':
														lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
														oldLine++
													case '\\':
													default:
														return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
													}
												}
												return lines, nil
											}()
											if err != nil {
												return patchFile{}, fmt.Errorf("%s: %w", display, err)
											}
											file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
												header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
												if h.Section != "" {
													header += " " + h.Section
												}
												return header
											}(), Lines: lines})
										}
										return file, nil
									}()
									if err != nil {
										return nil, err
									}
									files = append(files, file)
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							untracked, err := func() ([]patchFile, error) {
								out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
								if err != nil {
									return nil, err
								}
								var files []patchFile
								for _, path := range func() []string {
									var paths []string
									for path := range bytes.SplitSeq(out, []byte{0}) {
										if len(path) > 0 {
											paths = append(paths, string(path))
										}
									}
									return paths
								}() {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, include, err := func() (patchFile, bool, error) {
										full := filepath.Join(root, filepath.FromSlash(path))
										info, err := os.Lstat(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
										}
										if info.Mode()&os.ModeSymlink != 0 {
											target, err := os.Readlink(full)
											if err != nil {
												return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
											}
											return func() patchFile {
												var content string = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range target {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
												sourceLines := func() []string {
													var content string = content
													content = strings.TrimSuffix(content, "\n")
													if content == "" {
														return nil
													}
													return strings.Split(content, "\n")
												}()
												lines := make([]patchLine, 0, len(sourceLines))
												for i, line := range sourceLines {
													lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
												}
												return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range path {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
											}(), true, nil
										}
										if !info.Mode().IsRegular() {
											return patchFile{}, false, nil
										}
										if info.Size() > patchMaxFileBytes {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
										}
										content, err := os.ReadFile(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
										}
										if bytes.IndexByte(content, 0) >= 0 {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
										}
										return func() patchFile {
											var content string = func() string {
												var result strings.Builder
												for _, r := range string(content) {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}()
											sourceLines := func() []string {
												var content string = content
												content = strings.TrimSuffix(content, "\n")
												if content == "" {
													return nil
												}
												return strings.Split(content, "\n")
											}()
											lines := make([]patchLine, 0, len(sourceLines))
											for i, line := range sourceLines {
												lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
											}
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
										}(), true, nil
									}()
									if err != nil {
										return nil, err
									}
									if include {
										files = append(files, file)
									}
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							files = append(files, untracked...)
							slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
							if err := ctx.Err(); err != nil {
								return patchPatch{}, err
							}
							return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
						}
					}
				}))
			})(ctx)
		}
		if len(args) > 1 {
			return errors.New("usage: review-my-slop [code|comments]")
		}
		switch args[0] {
		case "code":
			return (func(ctx context.Context) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
					directory, err := os.Getwd()
					if err != nil {
						return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
					}
					{
						if kind != patchUnstaged && kind != patchBranch {
							return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
						}
						if err := ctx.Err(); err != nil {
							return patchPatch{}, err
						}
						root, err := func() (string, error) {
							rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
							if err != nil {
								return "", err
							}
							root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
							if err != nil {
								return "", fmt.Errorf("resolve repository root: %w", err)
							}
							return root, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						branch, err := func() (string, error) {
							out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
							if ctx.Err() != nil {
								return "", ctx.Err()
							}
							if err == nil {
								return strings.TrimSpace(string(out)), nil
							}
							for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
								_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
								if ctx.Err() != nil {
									return "", ctx.Err()
								}
								if err == nil {
									return candidate, nil
								}
							}
							return "", nil
						}()
						if err != nil {
							return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
						}
						revisions, readOld, err := func() ([]string, patchSourceReader, error) {
							if kind != patchBranch {
								return nil, func(ctx context.Context, run patchRunner, root, path string) string {
									if path == "" || path == "/dev/null" {
										return ""
									}
									out, err := run.Run(ctx, root, "show", ":"+path)
									if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}, nil
							}
							if branch == "" {
								return nil, nil, errors.New("default branch is unavailable")
							}
							baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
							if err != nil {
								return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
							}
							base := strings.TrimSpace(string(baseBytes))
							readOld := func(ctx context.Context, run patchRunner, root, path string) string {
								if path == "" || path == "/dev/null" {
									return ""
								}
								out, err := run.Run(ctx, root, "show", base+":"+path)
								if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}
							return []string{base}, readOld, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						raw, err := func() ([]byte, error) {
							args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
							args = append(args, revisions...)
							args = append(args, "--")
							return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
						}()
						if err != nil {
							return patchPatch{}, err
						}
						{
							files, err := func() ([]patchFile, error) {
								if len(bytes.TrimSpace(raw)) == 0 {
									return nil, nil
								}
								parsed, err := diff.ParseMultiFileDiff(raw)
								if err != nil {
									return nil, fmt.Errorf("parse git diff: %w", err)
								}
								files := make([]patchFile, 0, len(parsed))
								for _, fd := range parsed {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, err := func() (patchFile, error) {
										oldPath := func() string {
											var path string = fd.OrigName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										newPath := func() string {
											var path string = fd.NewName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										display := newPath
										if display == "" || display == "/dev/null" {
											display = oldPath
										}
										file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
											var value string = display
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}(), Metadata: func() []string {
											var values []string = fd.Extended
											result := make([]string, len(values))
											for index, value := range values {
												result[index] = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}
											return result
										}()}
										file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
										file.NewSource = func() string {
											if newPath == "" || newPath == "/dev/null" {
												return ""
											}
											full := filepath.Join(root, filepath.FromSlash(newPath))
											info, err := os.Lstat(full)
											if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
												return ""
											}
											out, err := os.ReadFile(full)
											if err != nil || bytes.IndexByte(out, 0) >= 0 {
												return ""
											}
											var result strings.Builder
											for _, r := range string(out) {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}()
										for _, h := range fd.Hunks {
											lines, err := func() ([]patchLine, error) {
												var (
													oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
													body             []byte = h.Body
												)
												rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
												if len(body) == 0 {
													return []patchLine{}, nil
												}
												lines := make([]patchLine, 0, len(rawLines))
												for _, raw := range rawLines {
													if len(raw) == 0 {
														return nil, errors.New("malformed empty diff line")
													}
													text := func() string {
														var value string = string(raw[1:])
														return strings.ReplaceAll(func() string {
															var result strings.Builder
															for _, r := range value {
																switch {
																case r == '\n' || r == '\t':
																	_, _ = result.WriteRune(r)
																case r == '\r':
																	_, _ = result.WriteString(`\r`)
																case r < 0x20 || r == 0x7f:
																	fmt.Fprintf(&result, `\x%02x`, r)
																default:
																	_, _ = result.WriteRune(r)
																}
															}
															return result.String()
														}(), "\n", `\n`)
													}()
													switch raw[0] {
													case ' ':
														lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
														oldLine++
														newLine++
													case '+':
														lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
														newLine++
													case '-':
														lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
														oldLine++
													case '\\':
													default:
														return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
													}
												}
												return lines, nil
											}()
											if err != nil {
												return patchFile{}, fmt.Errorf("%s: %w", display, err)
											}
											file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
												header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
												if h.Section != "" {
													header += " " + h.Section
												}
												return header
											}(), Lines: lines})
										}
										return file, nil
									}()
									if err != nil {
										return nil, err
									}
									files = append(files, file)
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							untracked, err := func() ([]patchFile, error) {
								out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
								if err != nil {
									return nil, err
								}
								var files []patchFile
								for _, path := range func() []string {
									var paths []string
									for path := range bytes.SplitSeq(out, []byte{0}) {
										if len(path) > 0 {
											paths = append(paths, string(path))
										}
									}
									return paths
								}() {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, include, err := func() (patchFile, bool, error) {
										full := filepath.Join(root, filepath.FromSlash(path))
										info, err := os.Lstat(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
										}
										if info.Mode()&os.ModeSymlink != 0 {
											target, err := os.Readlink(full)
											if err != nil {
												return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
											}
											return func() patchFile {
												var content string = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range target {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
												sourceLines := func() []string {
													var content string = content
													content = strings.TrimSuffix(content, "\n")
													if content == "" {
														return nil
													}
													return strings.Split(content, "\n")
												}()
												lines := make([]patchLine, 0, len(sourceLines))
												for i, line := range sourceLines {
													lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
												}
												return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range path {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
											}(), true, nil
										}
										if !info.Mode().IsRegular() {
											return patchFile{}, false, nil
										}
										if info.Size() > patchMaxFileBytes {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
										}
										content, err := os.ReadFile(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
										}
										if bytes.IndexByte(content, 0) >= 0 {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
										}
										return func() patchFile {
											var content string = func() string {
												var result strings.Builder
												for _, r := range string(content) {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}()
											sourceLines := func() []string {
												var content string = content
												content = strings.TrimSuffix(content, "\n")
												if content == "" {
													return nil
												}
												return strings.Split(content, "\n")
											}()
											lines := make([]patchLine, 0, len(sourceLines))
											for i, line := range sourceLines {
												lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
											}
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
										}(), true, nil
									}()
									if err != nil {
										return nil, err
									}
									if include {
										files = append(files, file)
									}
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							files = append(files, untracked...)
							slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
							if err := ctx.Err(); err != nil {
								return patchPatch{}, err
							}
							return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
						}
					}
				})(ctx, patchUnstaged)
				if err != nil {
					return err
				}
				return (func(
					ctx context.Context,
					currentPatch patchPatch,
					inlineLocalCommentStore1 commentsStore,
					refresh func(context.Context, patchKind) (patchPatch, error),
				) error {
					return (func(
						ctx context.Context,
						currentPatch patchPatch,
						store appCommentStore, refresh func(context.Context, patchKind) (patchPatch, error),
						terminalSize func() appSize, launch func(appModel, ...tea.ProgramOption) error,
					) error {
						pending, err := store.List(currentPatch.Root)
						if err != nil {
							return err
						}
						dimensions := terminalSize()
						m, err := func() (appModel, error) {
							preferences, err := (func() (settingsPreferences, error) {
								path, err := (func() (string, error) {
									config, err := os.UserConfigDir()
									if err != nil {
										return "", fmt.Errorf("resolve UI settings directory: %w", err)
									}
									return filepath.Join(config, "review-my-slop", "ui.json"), nil
								})()
								if err != nil {
									return settingsPreferences{}, err
								}
								return (func(path string) (settingsPreferences, error) {
									data, err := os.ReadFile(path)
									if errors.Is(err, os.ErrNotExist) {
										return settingsPreferences{}, nil
									}
									if err != nil {
										return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
									}
									var settings settingsPreferences
									if err := json.Unmarshal(data, &settings); err != nil {
										return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
									}
									return settings, nil
								})(path)
							})()
							if err != nil {
								return appModel{}, err
							}
							m := func() appModel {
								var layout appInitialLayout = appInitialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error {
									return (func(preferences settingsPreferences) error {
										path, err := (func() (string, error) {
											config, err := os.UserConfigDir()
											if err != nil {
												return "", fmt.Errorf("resolve UI settings directory: %w", err)
											}
											return filepath.Join(config, "review-my-slop", "ui.json"), nil
										})()
										if err != nil {
											return err
										}
										return (func(path string, preferences settingsPreferences) error {
											if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
												return fmt.Errorf("create UI settings directory: %w", err)
											}
											data, err := json.Marshal(preferences)
											if err != nil {
												return fmt.Errorf("encode UI settings: %w", err)
											}
											data = append(data, '\n')
											temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
											if err != nil {
												return fmt.Errorf("create UI settings file: %w", err)
											}
											temporaryPath := temporary.Name()
											defer os.Remove(temporaryPath)
											if err := temporary.Chmod(0o600); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("secure UI settings file: %w", err)
											}
											if _, err := temporary.Write(data); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("write UI settings: %w", err)
											}
											if err := temporary.Close(); err != nil {
												return fmt.Errorf("close UI settings: %w", err)
											}
											if err := os.Rename(temporaryPath, path); err != nil {
												return fmt.Errorf("replace UI settings: %w", err)
											}
											return nil
										})(path, preferences)
									})(settingsPreferences{SideBySide: enabled})
								}, size: dimensions}
								inlineLocalSize1 := layout.size
								if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
									inlineLocalSize1 = appDefaultSize
								}
								m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
									v := &commentscreenView{width: 80, height: 30}
									v.Update(items)
									return v
								})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
									comment.Repository = current.Root
									if comment.ID != "" {
										if err := store.Update(comment); err != nil {
											return commentsComment{}, err
										}
										return comment, nil
									}
									return store.Add(comment)
								}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
									m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
									for _, sequence := range sequences {
										sequence.Keys = append([]string(nil), sequence.Keys...)
										m.sequences[sequence.Prefix] = sequence
									}
									return m
								})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
								m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
									v := &diffscreenView{options: options, width: 80, height: 30}
									v.view = func() *diffscreenDiffView {
										{
											var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
											v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
											for fileIndex, file := range p.Files {
												var file patchFile = file
												v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
												for _, metadata := range file.Metadata {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
												}
												highlighted := func() diffscreenPair {
													var file *patchFile = &file
													theme := diffscreenLightTheme
													if v.dark {
														theme = diffscreenDarkTheme
													}
													return diffscreenPair{Old: func() []string {
														var filename, source, theme string = file.OldPath, file.OldSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}(), New: func() []string {
														var filename, source, theme string = file.NewPath, file.NewSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}()}
												}()
												for hunkIndex, hunk := range file.Hunks {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
														if strings.HasPrefix(hunk.Header, "@@") {
															return hunk.Header
														}
														return "@@ " + hunk.Header
													}()})
													for _, indices := range func() [][2]int {
														rows := make([][2]int, 0, len(hunk.Lines))
														for index := 0; index < len(hunk.Lines); {
															if !v.split || hunk.Lines[index].Kind == patchContext {
																rows = append(rows, [2]int{index, index})
																index++
																continue
															}
															if hunk.Lines[index].Kind == patchAddition {
																rows = append(rows, [2]int{-1, index})
																index++
																continue
															}
															addedStart := func() int {
																var start int = index
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
																	start++
																}
																return start
															}()
															end := func() int {
																var start int = addedStart
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
																	start++
																}
																return start
															}()
															rows = append(rows, func() [][2]int {
																count := max(addedStart-index, end-addedStart)
																rows := make([][2]int, 0, count)
																for offset := 0; offset < count; offset++ {
																	inlineLocalPair2 := [2]int{-1, -1}
																	if index+offset < addedStart {
																		inlineLocalPair2[0] = index + offset
																	}
																	if addedStart+offset < end {
																		inlineLocalPair2[1] = addedStart + offset
																	}
																	rows = append(rows, inlineLocalPair2)
																}
																return rows
															}()...)
															index = end
														}
														return rows
													}() {
														row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
														func() {
															var row *diffscreenEntry = &row
															if !v.split {
																row.text = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
																return
															}
															if row.leftLine >= 0 {
																row.left = func() string {
																	var line patchLine = hunk.Lines[row.leftLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
															if row.rightLine >= 0 {
																row.right = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
														}()
														v.rows = append(v.rows, row)
													}
												}
											}
											return v
										}
									}()
									v.viewport = func(v *diffscreenDiffView, width, height int) diffscreenDiffViewport {
										var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
										viewport.Width, viewport.Height = max(1, width), max(1, height)
										{
											var viewport diffscreenDiffViewport = viewport
											viewport.top = max(0, min(viewport.top, func() int {
												end := max(0, len(v.rows)-viewport.Height)
												if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
													end++
												}
												return end
											}()))
											viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
												contentWidth := max(1, viewport.Width-14)
												extra := 0
												if v.split {
													contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
												}
												longest := 0
												for _, current := range v.rows {
													if current.kind != diffscreenLineRow {
														continue
													}
													longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
												}
												return max(0, longest-contentWidth)
											}()))
											return viewport
										}
									}(v.view, v.width, (func(height int) int { return max(1, height-3) })(v.height))
									v.cursor, _ = func() (diffscreenDiffCursor, bool) {
										var v *diffscreenDiffView = v.view
										if cursor, ok := func() (diffscreenDiffCursor, bool) {
											step := int(diffscreenPrivateForward)
											for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
												if cursor, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return cursor, true
												}
											}
											return diffscreenDiffCursor{}, false
										}(); ok {
											return cursor, true
										}
										step := int(diffscreenPrivateForward)
										for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
											if cursor, ok := func() (diffscreenDiffCursor, bool) {
												cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return cursor, true
											}
										}
										return diffscreenDiffCursor{}, false
									}()
									return v
								})(currentPatch, m.diffOptions)
								m.diffView.Resize(m.width, m.height)
								m.commentView.Resize(m.width, m.height)
								return m
							}()
							m.delete = func(comment commentsComment, current patchPatch) error {
								return store.Delete(current.Root, comment.ID)
							}
							m.load = func() ([]commentsComment, error) { return store.List(currentPatch.Root) }
							return m, nil
						}()
						if err != nil {
							return err
						}
						m.ctx = ctx
						if refresh != nil {
							m.refresh = func(kind patchKind) (patchPatch, error) { return refresh(ctx, kind) }
						}
						return launch(
							m,
							tea.WithContext(ctx),
							tea.WithWindowSize(dimensions.Width, dimensions.Height),
						)
					})(
						ctx,
						currentPatch,
						inlineLocalCommentStore1,
						refresh,
						(func() appSize {
							if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							return appDefaultSize
						}),
						func(
							m appModel, options ...tea.ProgramOption,
						) error {
							_, err := tea.NewProgram(m, options...).Run()
							return err
						},
					)
				})(ctx, currentPatch, commentStore, (func(ctx context.Context, kind patchKind) (patchPatch, error) {
					directory, err := os.Getwd()
					if err != nil {
						return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
					}
					{
						if kind != patchUnstaged && kind != patchBranch {
							return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
						}
						if err := ctx.Err(); err != nil {
							return patchPatch{}, err
						}
						root, err := func() (string, error) {
							rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
							if err != nil {
								return "", err
							}
							root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
							if err != nil {
								return "", fmt.Errorf("resolve repository root: %w", err)
							}
							return root, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						branch, err := func() (string, error) {
							out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
							if ctx.Err() != nil {
								return "", ctx.Err()
							}
							if err == nil {
								return strings.TrimSpace(string(out)), nil
							}
							for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
								_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
								if ctx.Err() != nil {
									return "", ctx.Err()
								}
								if err == nil {
									return candidate, nil
								}
							}
							return "", nil
						}()
						if err != nil {
							return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
						}
						revisions, readOld, err := func() ([]string, patchSourceReader, error) {
							if kind != patchBranch {
								return nil, func(ctx context.Context, run patchRunner, root, path string) string {
									if path == "" || path == "/dev/null" {
										return ""
									}
									out, err := run.Run(ctx, root, "show", ":"+path)
									if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}, nil
							}
							if branch == "" {
								return nil, nil, errors.New("default branch is unavailable")
							}
							baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
							if err != nil {
								return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
							}
							base := strings.TrimSpace(string(baseBytes))
							readOld := func(ctx context.Context, run patchRunner, root, path string) string {
								if path == "" || path == "/dev/null" {
									return ""
								}
								out, err := run.Run(ctx, root, "show", base+":"+path)
								if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}
							return []string{base}, readOld, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						raw, err := func() ([]byte, error) {
							args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
							args = append(args, revisions...)
							args = append(args, "--")
							return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
						}()
						if err != nil {
							return patchPatch{}, err
						}
						{
							files, err := func() ([]patchFile, error) {
								if len(bytes.TrimSpace(raw)) == 0 {
									return nil, nil
								}
								parsed, err := diff.ParseMultiFileDiff(raw)
								if err != nil {
									return nil, fmt.Errorf("parse git diff: %w", err)
								}
								files := make([]patchFile, 0, len(parsed))
								for _, fd := range parsed {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, err := func() (patchFile, error) {
										oldPath := func() string {
											var path string = fd.OrigName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										newPath := func() string {
											var path string = fd.NewName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										display := newPath
										if display == "" || display == "/dev/null" {
											display = oldPath
										}
										file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
											var value string = display
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}(), Metadata: func() []string {
											var values []string = fd.Extended
											result := make([]string, len(values))
											for index, value := range values {
												result[index] = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}
											return result
										}()}
										file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
										file.NewSource = func() string {
											if newPath == "" || newPath == "/dev/null" {
												return ""
											}
											full := filepath.Join(root, filepath.FromSlash(newPath))
											info, err := os.Lstat(full)
											if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
												return ""
											}
											out, err := os.ReadFile(full)
											if err != nil || bytes.IndexByte(out, 0) >= 0 {
												return ""
											}
											var result strings.Builder
											for _, r := range string(out) {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}()
										for _, h := range fd.Hunks {
											lines, err := func() ([]patchLine, error) {
												var (
													oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
													body             []byte = h.Body
												)
												rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
												if len(body) == 0 {
													return []patchLine{}, nil
												}
												lines := make([]patchLine, 0, len(rawLines))
												for _, raw := range rawLines {
													if len(raw) == 0 {
														return nil, errors.New("malformed empty diff line")
													}
													text := func() string {
														var value string = string(raw[1:])
														return strings.ReplaceAll(func() string {
															var result strings.Builder
															for _, r := range value {
																switch {
																case r == '\n' || r == '\t':
																	_, _ = result.WriteRune(r)
																case r == '\r':
																	_, _ = result.WriteString(`\r`)
																case r < 0x20 || r == 0x7f:
																	fmt.Fprintf(&result, `\x%02x`, r)
																default:
																	_, _ = result.WriteRune(r)
																}
															}
															return result.String()
														}(), "\n", `\n`)
													}()
													switch raw[0] {
													case ' ':
														lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
														oldLine++
														newLine++
													case '+':
														lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
														newLine++
													case '-':
														lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
														oldLine++
													case '\\':
													default:
														return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
													}
												}
												return lines, nil
											}()
											if err != nil {
												return patchFile{}, fmt.Errorf("%s: %w", display, err)
											}
											file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
												header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
												if h.Section != "" {
													header += " " + h.Section
												}
												return header
											}(), Lines: lines})
										}
										return file, nil
									}()
									if err != nil {
										return nil, err
									}
									files = append(files, file)
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							untracked, err := func() ([]patchFile, error) {
								out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
								if err != nil {
									return nil, err
								}
								var files []patchFile
								for _, path := range func() []string {
									var paths []string
									for path := range bytes.SplitSeq(out, []byte{0}) {
										if len(path) > 0 {
											paths = append(paths, string(path))
										}
									}
									return paths
								}() {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, include, err := func() (patchFile, bool, error) {
										full := filepath.Join(root, filepath.FromSlash(path))
										info, err := os.Lstat(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
										}
										if info.Mode()&os.ModeSymlink != 0 {
											target, err := os.Readlink(full)
											if err != nil {
												return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
											}
											return func() patchFile {
												var content string = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range target {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
												sourceLines := func() []string {
													var content string = content
													content = strings.TrimSuffix(content, "\n")
													if content == "" {
														return nil
													}
													return strings.Split(content, "\n")
												}()
												lines := make([]patchLine, 0, len(sourceLines))
												for i, line := range sourceLines {
													lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
												}
												return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range path {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
											}(), true, nil
										}
										if !info.Mode().IsRegular() {
											return patchFile{}, false, nil
										}
										if info.Size() > patchMaxFileBytes {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
										}
										content, err := os.ReadFile(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
										}
										if bytes.IndexByte(content, 0) >= 0 {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
										}
										return func() patchFile {
											var content string = func() string {
												var result strings.Builder
												for _, r := range string(content) {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}()
											sourceLines := func() []string {
												var content string = content
												content = strings.TrimSuffix(content, "\n")
												if content == "" {
													return nil
												}
												return strings.Split(content, "\n")
											}()
											lines := make([]patchLine, 0, len(sourceLines))
											for i, line := range sourceLines {
												lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
											}
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
										}(), true, nil
									}()
									if err != nil {
										return nil, err
									}
									if include {
										files = append(files, file)
									}
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							files = append(files, untracked...)
							slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
							if err := ctx.Err(); err != nil {
								return patchPatch{}, err
							}
							return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
						}
					}
				}))
			})(ctx)
		case "comments":
			return (func(ctx context.Context, output io.Writer) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
					directory, err := os.Getwd()
					if err != nil {
						return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
					}
					{
						if kind != patchUnstaged && kind != patchBranch {
							return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
						}
						if err := ctx.Err(); err != nil {
							return patchPatch{}, err
						}
						root, err := func() (string, error) {
							rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
							if err != nil {
								return "", err
							}
							root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
							if err != nil {
								return "", fmt.Errorf("resolve repository root: %w", err)
							}
							return root, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						branch, err := func() (string, error) {
							out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
							if ctx.Err() != nil {
								return "", ctx.Err()
							}
							if err == nil {
								return strings.TrimSpace(string(out)), nil
							}
							for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
								_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
								if ctx.Err() != nil {
									return "", ctx.Err()
								}
								if err == nil {
									return candidate, nil
								}
							}
							return "", nil
						}()
						if err != nil {
							return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
						}
						revisions, readOld, err := func() ([]string, patchSourceReader, error) {
							if kind != patchBranch {
								return nil, func(ctx context.Context, run patchRunner, root, path string) string {
									if path == "" || path == "/dev/null" {
										return ""
									}
									out, err := run.Run(ctx, root, "show", ":"+path)
									if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}, nil
							}
							if branch == "" {
								return nil, nil, errors.New("default branch is unavailable")
							}
							baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
							if err != nil {
								return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
							}
							base := strings.TrimSpace(string(baseBytes))
							readOld := func(ctx context.Context, run patchRunner, root, path string) string {
								if path == "" || path == "/dev/null" {
									return ""
								}
								out, err := run.Run(ctx, root, "show", base+":"+path)
								if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}
							return []string{base}, readOld, nil
						}()
						if err != nil {
							return patchPatch{}, err
						}
						raw, err := func() ([]byte, error) {
							args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
							args = append(args, revisions...)
							args = append(args, "--")
							return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
						}()
						if err != nil {
							return patchPatch{}, err
						}
						{
							files, err := func() ([]patchFile, error) {
								if len(bytes.TrimSpace(raw)) == 0 {
									return nil, nil
								}
								parsed, err := diff.ParseMultiFileDiff(raw)
								if err != nil {
									return nil, fmt.Errorf("parse git diff: %w", err)
								}
								files := make([]patchFile, 0, len(parsed))
								for _, fd := range parsed {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, err := func() (patchFile, error) {
										oldPath := func() string {
											var path string = fd.OrigName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										newPath := func() string {
											var path string = fd.NewName
											path = strings.TrimSpace(path)
											path = strings.TrimPrefix(path, "a/")
											path = strings.TrimPrefix(path, "b/")
											if path == "/dev/null" {
												return ""
											}
											return path
										}()
										display := newPath
										if display == "" || display == "/dev/null" {
											display = oldPath
										}
										file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
											var value string = display
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}(), Metadata: func() []string {
											var values []string = fd.Extended
											result := make([]string, len(values))
											for index, value := range values {
												result[index] = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}
											return result
										}()}
										file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
										file.NewSource = func() string {
											if newPath == "" || newPath == "/dev/null" {
												return ""
											}
											full := filepath.Join(root, filepath.FromSlash(newPath))
											info, err := os.Lstat(full)
											if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
												return ""
											}
											out, err := os.ReadFile(full)
											if err != nil || bytes.IndexByte(out, 0) >= 0 {
												return ""
											}
											var result strings.Builder
											for _, r := range string(out) {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}()
										for _, h := range fd.Hunks {
											lines, err := func() ([]patchLine, error) {
												var (
													oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
													body             []byte = h.Body
												)
												rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
												if len(body) == 0 {
													return []patchLine{}, nil
												}
												lines := make([]patchLine, 0, len(rawLines))
												for _, raw := range rawLines {
													if len(raw) == 0 {
														return nil, errors.New("malformed empty diff line")
													}
													text := func() string {
														var value string = string(raw[1:])
														return strings.ReplaceAll(func() string {
															var result strings.Builder
															for _, r := range value {
																switch {
																case r == '\n' || r == '\t':
																	_, _ = result.WriteRune(r)
																case r == '\r':
																	_, _ = result.WriteString(`\r`)
																case r < 0x20 || r == 0x7f:
																	fmt.Fprintf(&result, `\x%02x`, r)
																default:
																	_, _ = result.WriteRune(r)
																}
															}
															return result.String()
														}(), "\n", `\n`)
													}()
													switch raw[0] {
													case ' ':
														lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
														oldLine++
														newLine++
													case '+':
														lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
														newLine++
													case '-':
														lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
														oldLine++
													case '\\':
													default:
														return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
													}
												}
												return lines, nil
											}()
											if err != nil {
												return patchFile{}, fmt.Errorf("%s: %w", display, err)
											}
											file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
												header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
												if h.Section != "" {
													header += " " + h.Section
												}
												return header
											}(), Lines: lines})
										}
										return file, nil
									}()
									if err != nil {
										return nil, err
									}
									files = append(files, file)
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							untracked, err := func() ([]patchFile, error) {
								out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
								if err != nil {
									return nil, err
								}
								var files []patchFile
								for _, path := range func() []string {
									var paths []string
									for path := range bytes.SplitSeq(out, []byte{0}) {
										if len(path) > 0 {
											paths = append(paths, string(path))
										}
									}
									return paths
								}() {
									if err := ctx.Err(); err != nil {
										return nil, err
									}
									file, include, err := func() (patchFile, bool, error) {
										full := filepath.Join(root, filepath.FromSlash(path))
										info, err := os.Lstat(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
										}
										if info.Mode()&os.ModeSymlink != 0 {
											target, err := os.Readlink(full)
											if err != nil {
												return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
											}
											return func() patchFile {
												var content string = strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range target {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
												sourceLines := func() []string {
													var content string = content
													content = strings.TrimSuffix(content, "\n")
													if content == "" {
														return nil
													}
													return strings.Split(content, "\n")
												}()
												lines := make([]patchLine, 0, len(sourceLines))
												for i, line := range sourceLines {
													lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
												}
												return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range path {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
											}(), true, nil
										}
										if !info.Mode().IsRegular() {
											return patchFile{}, false, nil
										}
										if info.Size() > patchMaxFileBytes {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
										}
										content, err := os.ReadFile(full)
										if err != nil {
											return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
										}
										if bytes.IndexByte(content, 0) >= 0 {
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
										}
										return func() patchFile {
											var content string = func() string {
												var result strings.Builder
												for _, r := range string(content) {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}()
											sourceLines := func() []string {
												var content string = content
												content = strings.TrimSuffix(content, "\n")
												if content == "" {
													return nil
												}
												return strings.Split(content, "\n")
											}()
											lines := make([]patchLine, 0, len(sourceLines))
											for i, line := range sourceLines {
												lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
											}
											return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range path {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
										}(), true, nil
									}()
									if err != nil {
										return nil, err
									}
									if include {
										files = append(files, file)
									}
								}
								return files, nil
							}()
							if err != nil {
								return patchPatch{}, err
							}
							files = append(files, untracked...)
							slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
							if err := ctx.Err(); err != nil {
								return patchPatch{}, err
							}
							return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
						}
					}
				})(ctx, patchUnstaged)
				if err != nil {
					return err
				}
				return commentStore.WritePending(output, currentPatch.Root)
			})(ctx, output)
		default:
			return fmt.Errorf(
				"unknown subcommand %q; usage: review-my-slop [code|comments]",
				args[0],
			)
		}
	})(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(mainErrorExitCode)
	}
}

type diffscreenRowKind uint8

const (
	diffscreenFileRow diffscreenRowKind = iota
	diffscreenMetadataRow
	diffscreenHunkRow
	diffscreenLineRow
)

type diffscreenEntry struct {
	kind              diffscreenRowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffscreenDiffView struct {
	patch patchPatch
	rows  []diffscreenEntry
	split bool
	dark  bool
}

type patchLine struct {
	Kind      patchLineKind
	Text      string
	OldNumber patchLineNumber
	NewNumber patchLineNumber
}

type patchLineNumber int

type patchLineKind uint8

const (
	patchContext patchLineKind = iota
	patchAddition
	patchDeletion
)

type editorCommandAdapter struct {
	run            func() error
	body           string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *editorCommandAdapter) Run() error            { return c.run() }
func (c *editorCommandAdapter) SetStdin(r io.Reader)  { c.stdin = r }
func (c *editorCommandAdapter) SetStdout(w io.Writer) { c.stdout = w }
func (c *editorCommandAdapter) SetStderr(w io.Writer) { c.stderr = w }

type diffscreenPair struct {
	Old []string
	New []string
}

const (
	diffscreenLightTheme = "catppuccin-latte"
	diffscreenDarkTheme  = "catppuccin-mocha"
)
