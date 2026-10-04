package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	commentscreen "github.com/eskelinenantti/review-my-slop/internal/ui/comments"
	patchscreen "github.com/eskelinenantti/review-my-slop/internal/ui/patch"
)

type Size struct {
	Width  int
	Height int
}

var DefaultSize = Size{Width: 80, Height: 30}

type Initial struct {
	Patch         patch.Patch
	Comments      []comments.Comment
	DefaultBranch string
	SideBySide    bool
	Size          Size
}

type Dependencies struct {
	Load       func(base string) (patch.Patch, error)
	List       func() ([]comments.Comment, error)
	Save       func(comments.Draft) (comments.Comment, error)
	Delete     func(id string) error
	SaveLayout func(bool) error
}

type screen uint8

const (
	patchScreen screen = iota
	commentsScreen
	helpScreen
)

type Model struct {
	patch    *patchscreen.Model
	comments *commentscreen.Model
	active   screen
	width    int
	height   int
	quitting bool
}

func New(initial Initial, dependencies Dependencies) *Model {
	size := initial.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	return &Model{
		patch: patchscreen.New(patchscreen.Initial{
			Patch:         initial.Patch,
			DefaultBranch: initial.DefaultBranch,
			SideBySide:    initial.SideBySide,
			Width:         size.Width,
			Height:        size.Height,
		}, patchscreen.Dependencies{
			Load:       dependencies.Load,
			SaveLayout: dependencies.SaveLayout,
		}),
		comments: commentscreen.New(commentscreen.Initial{
			Items:  initial.Comments,
			Width:  size.Width,
			Height: size.Height,
		}, commentscreen.Dependencies{
			List:   dependencies.List,
			Save:   dependencies.Save,
			Delete: dependencies.Delete,
		}),
		active: patchScreen,
		width:  size.Width,
		height: size.Height,
	}
}

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		return m, tea.Batch(m.patch.Update(msg), m.comments.Update(msg))
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
		return m, nil
	case patchscreen.CommentRequested:
		return m, m.comments.Begin(msg.Anchor)
	case patchscreen.CommentsRequested:
		m.active = commentsScreen
		return m, m.comments.Update(commentscreen.Show{})
	case patchscreen.HelpRequested:
		m.active = helpScreen
		return m, nil
	case patchscreen.QuitRequested:
		m.quitting = true
		return m, tea.Quit
	case patchscreen.CommentSaved:
		return m, m.patch.Update(msg)
	case commentscreen.BackRequested:
		m.active = patchScreen
		return m, nil
	case commentscreen.QuitRequested:
		m.quitting = true
		return m, tea.Quit
	case commentscreen.Saved:
		if msg.FromPatch {
			return m, m.patch.Update(patchscreen.CommentSaved{})
		}
		return m, nil
	case commentscreen.Cancelled:
		if msg.FromPatch {
			return m, m.patch.Update(patchscreen.CommentSaved{})
		}
		return m, nil
	case commentscreen.Failed:
		return m, m.patch.Update(patchscreen.Failure{Err: msg.Err})
	case tea.KeyPressMsg:
		if m.active == helpScreen {
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
				m.active = patchScreen
			}
			return m, nil
		}
		if m.active == commentsScreen {
			return m, m.comments.Update(msg)
		}
		return m, m.patch.Update(msg)
	default:
		// Async completions belong to one child; both screens ignore messages
		// they do not own, so they remain deliverable while that screen is idle.
		return m, tea.Batch(m.patch.Update(msg), m.comments.Update(msg))
	}
}

func (m *Model) Resize(width, height int) {
	m.width, m.height = width, height
	m.patch.Resize(width, height)
	m.comments.Resize(width, height)
}

func (m *Model) Render() string {
	switch m.active {
	case commentsScreen:
		return m.comments.Render()
	case helpScreen:
		return m.renderHelp()
	default:
		return m.patch.Render()
	}
}

func (m *Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	view := tea.NewView(m.Render())
	view.AltScreen = true
	view.ReportFocus = true
	return view
}

func (m *Model) renderHelp() string {
	bindings := []keyBinding{
		{"j/k, arrows", "move"}, {"h/l, left/right", "scroll horizontally"},
		{"Ctrl-w h/l/w", "switch side-by-side pane"}, {"0/$", "start/end of lines"},
		{"gg/G", "first/last changed line"}, {"zz/zt/zb", "center/top/bottom current line"},
		{"Ctrl-d/Ctrl-u", "half-page down/up"}, {"/", "search diff text"},
		{"n/N", "next/previous search match"}, {"]f/[f", "next/previous file"},
		{"v", "select a line range"}, {"c", "comment on selection/current line"},
		{"e", "open current line in $EDITOR"}, {"C", "view comments"},
		{"R", "refresh diff"}, {"Tab", "toggle local/branch changes"},
		{"t", "toggle unified/side-by-side"}, {"q", "quit"},
	}
	keys := make([]keyBinding, 0, len(bindings))
	for _, binding := range bindings {
		binding.keys = strings.TrimSpace(binding.keys)
		keys = append(keys, binding)
	}
	body := append([]string{""}, renderKeyBindings(keys)...)
	return m.renderShellScreen(titleStyle.Render("review-my-slop help"), body, mutedStyle.Render("? or Esc closes help"))
}

type keyBinding struct{ keys, description string }

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	lines := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		lines = append(lines, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
	return lines
}

func (m *Model) renderShellScreen(header string, body []string, footer string) string {
	height := max(1, m.height-3)
	if len(body) > height {
		body = body[:height]
	}
	for len(body) < height {
		body = append(body, "")
	}
	footer = ansi.Truncate(footer, m.width, "")
	lines := make([]string, 0, height+3)
	lines = append(lines, header)
	lines = append(lines, body...)
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
)
