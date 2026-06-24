package neovim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"

	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/gitdiff"
	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

const (
	MethodInitCodeComment    = "review-my-slop/v1/code/comment/init"
	MethodSubmitCodeComment  = "review-my-slop/v1/code/comment/submit"
	MethodDiscardCodeComment = "review-my-slop/v1/code/comment/discard"
)

type commentStore interface {
	Save(review.Comment, string) (review.Comment, error)
}

type session struct {
	anchor     review.Anchor
	repository string
}

type Host struct {
	ctx      context.Context
	store    commentStore
	mu       sync.Mutex
	sessions map[nvim.Buffer]session
	nextID   atomic.Uint64
}

func NewHost(ctx context.Context, store commentStore) *Host {
	return &Host{ctx: ctx, store: store, sessions: make(map[nvim.Buffer]session)}
}

func NewDefaultHost(ctx context.Context) (*Host, error) {
	store, err := inbox.OpenDefault()
	if err != nil {
		return nil, err
	}
	return NewHost(ctx, store), nil
}

func (h *Host) Register(p *plugin.Plugin) {
	p.Handle(MethodInitCodeComment, h.initCodeComment)
	p.Handle(MethodSubmitCodeComment, h.submitCodeComment)
	p.Handle(MethodDiscardCodeComment, h.discardCodeComment)
}

func (h *Host) initCodeComment(v *nvim.Nvim, selected [2]int) error {
	if selected[0] < 1 || selected[1] < selected[0] {
		return errors.New("invalid line range")
	}
	source, err := v.CurrentBuffer()
	if err != nil {
		return fmt.Errorf("get current buffer: %w", err)
	}
	name, err := v.BufferName(source)
	if err != nil {
		return fmt.Errorf("get buffer name: %w", err)
	}
	if name == "" {
		return errors.New("save the file before commenting")
	}
	var cwd string
	if err := v.Eval("getcwd()", &cwd); err != nil {
		return fmt.Errorf("get Neovim working directory: %w", err)
	}
	path, err := absolutePath(cwd, name)
	if err != nil {
		return err
	}
	lines, err := v.BufferLines(source, selected[0]-1, selected[1], true)
	if err != nil {
		return fmt.Errorf("read selected lines: %w", err)
	}
	anchor, repository, err := buildAnchor(h.ctx, path, selected, lines)
	if err != nil {
		return err
	}

	buffer, err := v.CreateBuffer(false, true)
	if err != nil {
		return fmt.Errorf("create code comment buffer: %w", err)
	}
	bufferName := fmt.Sprintf("review-my-slop://comment/%d", h.nextID.Add(1))
	if err := v.SetBufferName(buffer, bufferName); err != nil {
		return fmt.Errorf("name code comment buffer: %w", err)
	}
	draft := editor.CommentDraft("", anchor)
	if err := v.SetBufferLines(buffer, 0, -1, true, draftLines(draft)); err != nil {
		return fmt.Errorf("populate code comment buffer: %w", err)
	}
	for option, value := range map[string]any{
		"bufhidden": "wipe",
		"buftype":   "acwrite",
		"filetype":  "markdown",
		"swapfile":  false,
	} {
		if err := v.SetBufferOption(buffer, option, value); err != nil {
			return fmt.Errorf("set code comment buffer %s: %w", option, err)
		}
	}

	h.mu.Lock()
	h.sessions[buffer] = session{anchor: anchor, repository: repository}
	h.mu.Unlock()
	if err := v.Command("tab split"); err != nil {
		h.forget(buffer)
		return fmt.Errorf("open code comment tab: %w", err)
	}
	if err := v.SetCurrentBuffer(buffer); err != nil {
		h.forget(buffer)
		return fmt.Errorf("show code comment buffer: %w", err)
	}
	window, err := v.CurrentWindow()
	if err != nil {
		return fmt.Errorf("get comment window: %w", err)
	}
	if err := v.SetWindowCursor(window, [2]int{1, 0}); err != nil {
		return fmt.Errorf("position comment cursor: %w", err)
	}
	return nil
}

func (h *Host) submitCodeComment(v *nvim.Nvim, bufferNumber int) error {
	buffer := nvim.Buffer(bufferNumber)
	h.mu.Lock()
	current, ok := h.sessions[buffer]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("code comment buffer %d is no longer active", buffer)
	}
	lines, err := v.BufferLines(buffer, 0, -1, true)
	if err != nil {
		return fmt.Errorf("read code comment buffer: %w", err)
	}
	body := editor.StripUnchangedSuggestion(joinLines(lines), current.anchor.QuotedLines)
	body = strings.TrimSpace(body)
	if body != "" {
		_, err = h.store.Save(review.Comment{
			Anchor: current.anchor,
			Body:   body,
		}, current.repository)
		if err != nil {
			return fmt.Errorf("save comment: %w", err)
		}
	}
	if err := v.SetBufferOption(buffer, "modified", false); err != nil {
		return fmt.Errorf("mark comment saved: %w", err)
	}
	h.forget(buffer)
	return nil
}

func (h *Host) discardCodeComment(bufferNumber int) {
	h.forget(nvim.Buffer(bufferNumber))
}

func (h *Host) forget(buffer nvim.Buffer) {
	h.mu.Lock()
	delete(h.sessions, buffer)
	h.mu.Unlock()
}

func RegisterDefault(ctx context.Context, p *plugin.Plugin) error {
	host, err := NewDefaultHost(ctx)
	if err != nil {
		return err
	}
	host.Register(p)
	return nil
}

func Serve(ctx context.Context, input io.Reader, output io.Writer, closer io.Closer) error {
	v, err := nvim.New(input, output, closer, nil)
	if err != nil {
		return err
	}
	p := plugin.New(v)
	if err := RegisterDefault(ctx, p); err != nil {
		return err
	}
	return v.Serve()
}

func buildAnchor(ctx context.Context, path string, selected [2]int, lines [][]byte) (review.Anchor, string, error) {
	repository, err := (gitdiff.Loader{}).Root(ctx, filepath.Dir(path))
	if err != nil {
		return review.Anchor{}, "", err
	}
	relative, err := filepath.Rel(repository, path)
	if err != nil {
		return review.Anchor{}, "", fmt.Errorf("find repository-relative path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return review.Anchor{}, "", errors.New("current file is outside the repository")
	}
	quoted := make([]string, len(lines))
	for index, line := range lines {
		quoted[index] = " " + string(line)
	}
	return review.Anchor{
		FilePath:    filepath.ToSlash(relative),
		NewStart:    selected[0],
		NewEnd:      selected[1],
		QuotedLines: quoted,
	}, repository, nil
}

func absolutePath(cwd, name string) (string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(cwd, name)
	}
	path, err := filepath.Abs(name)
	if err != nil {
		return "", fmt.Errorf("resolve buffer path: %w", err)
	}
	return filepath.Clean(path), nil
}

func draftLines(draft string) [][]byte {
	return byteLines(strings.TrimSuffix(draft, "\n"))
}

func joinLines(lines [][]byte) string {
	parts := make([]string, len(lines))
	for index, line := range lines {
		parts[index] = string(line)
	}
	return strings.Join(parts, "\n") + "\n"
}

func byteLines(value string) [][]byte {
	parts := strings.Split(value, "\n")
	lines := make([][]byte, len(parts))
	for index, part := range parts {
		lines[index] = []byte(part)
	}
	return lines
}
