package diffscreen

import (
	"reflect"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderedLineStylePrecedence(t *testing.T) {
	candidate := diffCursor{row: 2, pane: right}
	other := diffCursor{row: 3, pane: right}
	selected := beginSelection(candidate)
	unselected := beginSelection(other)
	for _, dark := range []bool{false, true} {
		styles := newLineStyles(dark)
		for _, kind := range []patch.LineKind{
			patch.Context, patch.Addition, patch.Deletion,
		} {
			var base lipgloss.Style
			switch kind {
			case patch.Addition:
				base = styles.added
			case patch.Deletion:
				base = styles.removed
			default:
				base = contextStyle
			}
			for _, tt := range []struct {
				name      string
				cursor    diffCursor
				selection *diffSelection
				style     lipgloss.Style
				colors    foregroundPolicy
			}{
				{"base", other, nil, base, preserveForeground},
				{
					"outside selection", other, &unselected,
					base, preserveForeground,
				},
				{
					"selection overrides base", other, &selected,
					styles.selection, discardForeground,
				},
				{
					"cursor overrides base", candidate, nil,
					cursorStyle, discardForeground,
				},
				{
					"cursor overrides selection", candidate, &selected,
					cursorStyle, discardForeground,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					style, colors := renderedLineStyle(
						kind, candidate, tt.cursor, tt.selection, styles,
					)
					if !reflect.DeepEqual(style, tt.style) {
						t.Fatalf("unexpected style: dark=%v kind=%v",
							dark, kind)
					}
					testutil.RequireEqual(t, colors, tt.colors,
						"dark=%v kind=%v", dark, kind)
				})
			}
		}
	}
}
