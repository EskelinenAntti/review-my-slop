package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
)

func newTestDiffscreenView(
	p patchPatch,
	options diffscreenOptions,
) *diffscreenView {
	v := &diffscreenView{options: options, width: 80, height: 30}
	v.view = buildTestDiffProjection(p, v)
	v.viewport = buildTestDiffViewport(v.view, v.width, max(1, v.height-3))
	v.cursor, _ = firstTestDiffCursor(v.view)
	return v
}

func buildTestDiffProjection(
	p patchPatch,
	screen *diffscreenView,
) *diffscreenDiffView {
	options := diffscreenOptions{
		SideBySide: screen.options.SideBySide && screen.width >= 100,
		Dark:       screen.options.Dark,
	}
	view := &diffscreenDiffView{
		patch: p,
		split: options.SideBySide,
		dark:  options.Dark,
	}
	for fileIndex, file := range p.Files {
		appendTestFileRows(view, fileIndex, file)
		highlighted := highlightTestPatchFile(file, view)
		for hunkIndex, hunk := range file.Hunks {
			appendTestHunkRows(view, fileIndex, hunkIndex, hunk, highlighted)
		}
	}
	return view
}

func appendTestFileRows(view *diffscreenDiffView, index int, file patchFile) {
	view.rows = append(view.rows, diffscreenEntry{
		kind: diffscreenFileRow, file: index, hunk: -1,
		leftLine: -1, rightLine: -1, text: file.DisplayPath,
	})
	for _, metadata := range file.Metadata {
		view.rows = append(view.rows, diffscreenEntry{
			kind: diffscreenMetadataRow, file: index, hunk: -1,
			leftLine: -1, rightLine: -1, text: metadata,
		})
	}
}

func appendTestHunkRows(
	view *diffscreenDiffView,
	fileIndex, hunkIndex int,
	hunk patchHunk,
	highlighted diffscreenPair,
) {
	header := hunk.Header
	if !strings.HasPrefix(header, "@@") {
		header = "@@ " + header
	}
	view.rows = append(view.rows, diffscreenEntry{
		kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex,
		leftLine: -1, rightLine: -1, text: header,
	})
	for _, pair := range testDiffLinePairs(hunk.Lines, view) {
		view.rows = append(view.rows, testDiffEntryForPair(
			fileIndex, hunkIndex, hunk.Lines, pair, view, highlighted,
		))
	}
}

func testDiffEntryForPair(
	fileIndex, hunkIndex int,
	lines []patchLine,
	pair [2]int,
	view *diffscreenDiffView,
	highlighted diffscreenPair,
) diffscreenEntry {
	row := diffscreenEntry{
		kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex,
		leftLine: pair[0], rightLine: pair[1],
	}
	if !view.split {
		row.text = testHighlightedLine(lines[row.rightLine], highlighted)
		return row
	}
	if row.leftLine >= 0 {
		row.left = testHighlightedLine(lines[row.leftLine], highlighted)
	}
	if row.rightLine >= 0 {
		row.right = testHighlightedLine(lines[row.rightLine], highlighted)
	}
	return row
}

func testDiffLinePairs(lines []patchLine, view *diffscreenDiffView) [][2]int {
	var pairs [][2]int
	for index := 0; index < len(lines); {
		paired, next := pairTestLine(view, lines, index)
		pairs = append(pairs, paired...)
		index = next
	}
	return pairs
}

func pairTestLine(
	view *diffscreenDiffView,
	lines []patchLine,
	index int,
) ([][2]int, int) {
	if !view.split || lines[index].Kind == patchContext {
		return [][2]int{{index, index}}, index + 1
	}
	if lines[index].Kind == patchAddition {
		return [][2]int{{-1, index}}, index + 1
	}
	deletedEnd := testChangeBlockEnd(lines, index, patchDeletion)
	addedEnd := testChangeBlockEnd(lines, deletedEnd, patchAddition)
	return pairTestChanges(index, deletedEnd, addedEnd), addedEnd
}

func testChangeBlockEnd(lines []patchLine, start int, kind patchLineKind) int {
	for start < len(lines) && lines[start].Kind == kind {
		start++
	}
	return start
}

func pairTestChanges(start, deletedEnd, addedEnd int) [][2]int {
	count := max(deletedEnd-start, addedEnd-deletedEnd)
	pairs := make([][2]int, 0, count)
	for offset := 0; offset < count; offset++ {
		pair := [2]int{-1, -1}
		if start+offset < deletedEnd {
			pair[0] = start + offset
		}
		if deletedEnd+offset < addedEnd {
			pair[1] = deletedEnd + offset
		}
		pairs = append(pairs, pair)
	}
	return pairs
}

func highlightTestPatchFile(
	file patchFile,
	view *diffscreenDiffView,
) diffscreenPair {
	theme := diffscreenLightTheme
	if view.dark {
		theme = diffscreenDarkTheme
	}
	return diffscreenPair{
		Old: highlightTestSource(file.OldPath, file.OldSource, theme),
		New: highlightTestSource(file.NewPath, file.NewSource, theme),
	}
}

func highlightTestSource(filename, source, theme string) []string {
	if source == "" {
		return nil
	}
	var output bytes.Buffer
	err := quick.Highlight(&output, source, filename, "terminal16m", theme)
	if err != nil {
		return splitTestSource(source)
	}
	rendered, err := io.ReadAll(&output)
	if err != nil {
		return splitTestSource(source)
	}
	return splitTestSource(string(rendered))
}

func splitTestSource(source string) []string {
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}

func testHighlightedLine(line patchLine, highlighted diffscreenPair) string {
	if line.Kind == patchDeletion {
		return highlightedLineOrSource(int(
			line.OldNumber,
		), line.Text, highlighted.Old)
	}
	return highlightedLineOrSource(int(
		line.NewNumber,
	), line.Text, highlighted.New)
}

func highlightedLineOrSource(
	number int,
	source string,
	highlighted []string,
) string {
	if number <= 0 || number > len(highlighted) {
		return source
	}
	return highlighted[number-1]
}

func buildTestDiffViewport(
	view *diffscreenDiffView,
	width, height int,
) diffscreenDiffViewport {
	viewport := diffscreenDiffViewport{
		Width:  max(1, width),
		Height: max(1, height),
	}
	viewport.top = testViewportTop(view.rows, viewport.Height)
	viewport.LeftColumn = testViewportLeftColumn(view, viewport.Width)
	return viewport
}

func testViewportTop(rows []diffscreenEntry, height int) int {
	end := max(0, len(rows)-height)
	if height > 1 && end >= 0 && end < len(
		rows,
	) && rows[end].kind != diffscreenFileRow {
		end++
	}
	return max(0, min(0, end))
}

func testViewportLeftColumn(view *diffscreenDiffView, width int) int {
	contentWidth, extra := max(1, width-14), 0
	if view.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, row := range view.rows {
		if row.kind != diffscreenLineRow {
			continue
		}
		text := ansi.Strip(row.text + row.left + row.right)
		longest = max(longest, lipgloss.Width(strings.ReplaceAll(
			text,
			"\t",
			"    ",
		))+extra)
	}
	return max(0, min(0, longest-contentWidth))
}

func firstTestDiffCursor(
	view *diffscreenDiffView,
) (diffscreenDiffCursor, bool) {
	if cursor, ok := firstTestCursorInPane(view, diffscreenRight); ok {
		return cursor, true
	}
	return firstTestCursorInPane(view, diffscreenLeft)
}

func firstTestCursorInPane(
	view *diffscreenDiffView,
	pane diffscreenDiffPane,
) (diffscreenDiffCursor, bool) {
	for rowIndex := range view.rows {
		cursor := diffscreenDiffCursor{row: rowIndex, pane: pane}
		if testCursorLine(view, cursor) >= 0 {
			return cursor, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func testCursorLine(view *diffscreenDiffView, cursor diffscreenDiffCursor) int {
	if cursor.row < 0 || cursor.row >= len(
		view.rows,
	) || view.rows[cursor.row].kind != diffscreenLineRow {
		return -1
	}
	row := view.rows[cursor.row]
	if !view.split || cursor.pane == diffscreenRight {
		return row.rightLine
	}
	return row.leftLine
}

func renderTestHelpBindings(
	bindings []helpscreenBinding,
	width, height int,
) string {
	return renderHelpBindings(bindings, width, height)
}

func expectedTestHelpBindings() []helpscreenBinding {
	return []helpscreenBinding{{
		Keys: "j/k, arrows", Description: "move",
	}, {
		Keys: "h/l, left/right", Description: "scroll horizontally",
	}, {
		Keys: "Mouse wheel", Description: "scroll content",
	}, {
		Keys: "Click/drag", Description: "focus line/select range",
	}, {
		Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane",
	}, {
		Keys: "0/$", Description: "start/end of lines",
	}, {
		Keys: "gg/G", Description: "first/last changed line",
	}, {
		Keys: "zz/zt/zb", Description: "center/top/bottom current line",
	}, {
		Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up",
	}, {
		Keys: "/", Description: "search diff text",
	}, {
		Keys: "n/N", Description: "next/previous search match",
	}, {
		Keys: "]f/[f", Description: "next/previous file",
	}, {
		Keys: "v", Description: "select a line range",
	}, {
		Keys: "c", Description: "comment on selection/current line",
	}, {
		Keys: "e", Description: "open current line in $EDITOR",
	}, {
		Keys: "C", Description: "view comments",
	}, {
		Keys: "R", Description: "refresh diff",
	}, {
		Keys: "Tab", Description: "toggle local/branch changes",
	}, {
		Keys: "t", Description: "toggle unified/side-by-side",
	}, {
		Keys: "q", Description: "quit",
	}}
}

func buildTestKeymapMatcher(sequences ...keymapSequence) keymapMatcher {
	m := keymapMatcher{sequences: make(
		map[string]keymapSequence,
		len(sequences),
	)}
	for _, sequence := range sequences {
		sequence.Keys = append([]string(nil), sequence.Keys...)
		m.sequences[sequence.Prefix] = sequence
	}
	return m
}

func newTestCommentScreen(items []commentsComment) *commentscreenView {
	v := &commentscreenView{width: 80, height: 30}
	v.Update(items)
	return v
}

func testFenceBacktickRun(lines []string) string {
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
}

func makeTestPatchWithManyLines() patchPatch {
	lines := make([]patchLine, 30)
	for index := range lines {
		lines[index] = patchLine{
			Kind: patchContext,
			Text: fmt.Sprintf(
				"line %d %s", index, strings.Repeat("x", 80),
			),
			OldNumber: patchLineNumber(index + 1),
			NewNumber: patchLineNumber(index + 1),
		}
	}
	return patchPatch{Files: []patchFile{{
		DisplayPath: "long.go",
		Hunks:       []patchHunk{{Header: "@@", Lines: lines}},
	}}}
}

func writeTestSideBySidePreference(enabled bool) error {
	path, err := testUIPreferencesPath()
	if err != nil {
		return err
	}
	return writeTestUIPreferences(path, settingsPreferences{
		SideBySide: enabled,
	})
}

func testUIPreferencesPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}

func writeTestUIPreferences(
	path string,
	preferences settingsPreferences,
) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create UI settings directory: %w", err)
	}
	data, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("encode UI settings: %w", err)
	}
	return replaceTestUIPreferences(path, data)
}

func replaceTestUIPreferences(path string, data []byte) error {
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
}

func newTestBasicAppModel() appModel {
	return newTestAppModel(newTestBasicPatch())
}

func newTestAppModel(p patchPatch) appModel {
	layout := appInitialLayout{size: appSize{Width: 100, Height: 30}}
	return newTestAppModelWithLayout(p, layout)
}

func newTestAppModelWithLayout(
	p patchPatch,
	layout appInitialLayout,
) appModel {
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = appDefaultSize
	}
	m := appModel{
		ctx:          context.Background(),
		currentPatch: p,
		comments:     appCommentState{},
		commentView:  newTestCommentScreen(nil),
		edit:         appCommentEdit{index: -1},
		width:        size.Width,
		height:       size.Height,
		saveLayout:   layout.SaveSideBySide,
		diffOptions: diffscreenOptions{
			SideBySide: layout.SideBySide,
			Dark:       true,
		},
		kind: p.Kind,
		keys: buildTestKeymapMatcher(
			keymapSequence{Prefix: "g", Keys: []string{
				"g",
			}, RetryUnmatched: true},
			keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymapSequence{Prefix: "[", Keys: []string{"f"}},
			keymapSequence{Prefix: "]", Keys: []string{"f"}},
			keymapSequence{Prefix: "ctrl+w", Keys: []string{
				"h",
				"l",
				"ctrl+w",
			}},
		),
	}
	m.diffView = newTestDiffscreenView(p, m.diffOptions)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	return m
}

func updateTestAppModel(
	t *testing.T,
	model appModel,
	message tea.Msg,
) appModel {
	t.Helper()
	next, _ := model.Update(message)
	result, ok := next.(appModel)
	requireEqual(t, ok, true, "model=%T", next)
	return result
}

func appModelFromTestResult(t *testing.T, result tea.Model) appModel {
	t.Helper()
	model, ok := result.(appModel)
	requireEqual(t, ok, true, "model=%T", result)
	return model
}

func runTestAppCommand(t *testing.T, model appModel, command tea.Cmd) appModel {
	t.Helper()
	return updateTestAppModel(t, model, command())
}

func pressTestKey(t *testing.T, model appModel, text string) appModel {
	t.Helper()
	runes := []rune(text)
	return updateTestAppModel(t, model, tea.KeyPressMsg(tea.Key{
		Text: text,
		Code: runes[0],
	}))
}

func testKeyMessage(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func currentTestDiffLineText(model appModel) string {
	_, line, _ := model.diffView.Current()
	return line.Text
}

func testRenderedDiffBody(model appModel) string {
	lines := strings.Split(model.diffView.Render(nil), "\n")
	return strings.Join(lines[1:len(lines)-2], "\n")
}

func moveTestDiffToLine(t *testing.T, model *appModel, text string) {
	t.Helper()
	model.diffView.Move(diffscreenFirstLine)
	for range testPatchLineCount(model.currentPatch) {
		if currentTestDiffLineText(*model) == text {
			return
		}
		model.diffView.Move(diffscreenNextLine)
	}
	t.Fatalf("line %q not found", text)
}

func testPatchLineCount(patch patchPatch) int {
	count := 0
	for _, file := range patch.Files {
		for _, hunk := range file.Hunks {
			count += len(hunk.Lines)
		}
	}
	return count
}

func newTestBasicPatch() patchPatch {
	return patchPatch{Root: "/repo", Files: []patchFile{{
		DisplayPath: "main.go",
		OldPath:     "main.go",
		NewPath:     "main.go",
		OldSource:   "package main\nold()\nkeep()\n",
		NewSource:   "package main\nnew()\nkeep()\nmore()\n",
		Hunks: []patchHunk{
			{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "package main",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patchDeletion,
						Text:      "old()",
						OldNumber: 2,
					},
					{
						Kind:      patchAddition,
						Text:      "new()",
						NewNumber: 2,
					},
					{
						Kind:      patchContext,
						Text:      "keep()",
						OldNumber: 3,
						NewNumber: 3,
					},
				},
			},
			{Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{
				Kind:      patchContext,
				Text:      "keep()",
				OldNumber: 3,
				NewNumber: 3,
			}, {
				Kind:      patchAddition,
				Text:      "more()",
				NewNumber: 4,
			}}},
		},
	}}}
}

func createTestCommentDraft(anchor commentsAnchor, body string,

) (string, error) {
	return createCommentDraft(body, anchor)
}

func readTestCommentDraft(anchor commentsAnchor, path string,

) (string, error) {
	return readCommentDraft(path, anchor)
}

func renderTestAppMode(m appModel) string {
	return m.View().Content
}

func newTestEditorCommandAdapter(
	ctx context.Context,
	command, body string,
	anchor commentsAnchor,
	onComplete func(string, error) tea.Msg,
) (*editorCommandAdapter, tea.ExecCallback) {
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		path, err := createTestCommentDraft(anchor, body)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		if err := runCommentEditor(ctx, command, path, adapter); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readTestCommentDraft(anchor, path)
		return err
	}
	return adapter, func(err error) tea.Msg {
		return onComplete(adapter.body, err)
	}
}

func readTestUIPreferences() (settingsPreferences, error) {
	path, err := testUIPreferencesPath()
	if err != nil {
		return settingsPreferences{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settingsPreferences{}, nil
	}
	if err != nil {
		return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
	}
	return decodeTestUIPreferences(data)
}

func decodeTestUIPreferences(data []byte) (settingsPreferences, error) {
	var settings settingsPreferences
	if err := json.Unmarshal(data, &settings); err != nil {
		return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
	}
	return settings, nil
}

func renderAppWorkflowMode(m *appModel) string {
	return renderTestAppMode(*m)
}

func updateTestAppWithKey(m *appModel, t *testing.T) appModel {
	t.Helper()
	return pressTestKey(t, *m, "t")
}

func pressTestAppWithG(m *appModel, t *testing.T) appModel {
	t.Helper()
	return pressTestKey(t, *m, "g")
}
