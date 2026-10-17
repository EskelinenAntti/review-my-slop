package main

import (
	strings "strings"
	testing "testing"

	lipgloss "charm.land/lipgloss/v2"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestHelpscreenRenderAlignsBindingsAndKeepsHintAtBottom(t *testing.T) {
	lines := strings.Split(
		ansi.Strip((func(bindings []helpscreenBinding, width, height int) string {
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
		})([]helpscreenBinding{{"x", "short"}, {"long", "wide"}}, 80, 8)),
		"\n",
	)
	if strings.Index(lines[2], "short") != strings.Index(lines[3], "wide") {
		t.Fatalf("unaligned rows: %q", lines)
	}
	if len(lines) != 8 || !strings.Contains(lines[6], "? or Esc closes help") {
		t.Fatalf("misplaced hint: %q", lines)
	}
}

func TestHelpscreenRenderEmptyAndTruncatedBindings(t *testing.T) {
	for _, height := range []int{1, 4, 8} {
		result := (func(bindings []helpscreenBinding, width, height int) string {
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
		})(nil, 80, height)
		if !strings.Contains(result, "? or Esc closes help") {
			t.Fatalf("missing hint: %q", result)
		}
	}
	result := (func(bindings []helpscreenBinding, width, height int) string {
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
	})([]helpscreenBinding{{"one", "shown"}, {"two", "hidden"}}, 80, 5)
	if !strings.Contains(
		result,
		"shown",
	) || strings.Contains(result, "hidden") {
		t.Fatalf("body not truncated: %q", result)
	}
}

func TestHelpscreenRenderClipsWideBindingText(t *testing.T) {
	rendered := (func(bindings []helpscreenBinding, width, height int) string {
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
	})(
		[]helpscreenBinding{{"界", strings.Repeat("description", 10)}},
		24,
		8,
	)
	for _, line := range strings.Split(rendered, "\n") {
		if ansi.StringWidth(line) > 24 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}
