package diffparse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

type Difftastic struct{}

func (Difftastic) Parse(lines []string) []Line {
	rows := difftasticRows(lines)
	parsed := make([]Line, len(lines))
	for i, line := range lines {
		parsed[i] = Line{Text: line}
		if _, ok := parseHeader(tui.StripANSI(line)); ok {
			parsed[i].Header = true
		}
		if row, ok := rows[i]; ok {
			parsed[i].Location = row.Location
			parsed[i].Selectable = true
			parsed[i].Editable = row.Editable
		}
	}
	return parsed
}

var numberRE = regexp.MustCompile(`\d+`)

type parsedRow struct {
	Location Location
	Editable bool
}

func difftasticRows(lines []string) map[int]parsedRow {
	rows := map[int]parsedRow{}
	file := ""
	for i, raw := range lines {
		plain := tui.StripANSI(raw)
		if header, ok := parseHeader(plain); ok {
			file = header
			continue
		}
		if file == "" {
			continue
		}
		line, editable, ok := parseLineNumber(plain, raw)
		if !ok {
			continue
		}
		rows[i] = parsedRow{
			Location: Location{File: file, Line: line},
			Editable: editable,
		}
	}
	return rows
}

func parseHeader(line string) (string, bool) {
	left, _, ok := strings.Cut(line, " --- ")
	if !ok {
		return "", false
	}
	file := strings.TrimSpace(left)
	if file == "" || strings.ContainsAny(file, " \t") {
		return "", false
	}
	if strings.HasPrefix(file, "a/") || strings.HasPrefix(file, "b/") {
		file = file[2:]
	}
	return file, true
}

func parseLineNumber(plain, raw string) (int, bool, bool) {
	fields := numberRE.FindAllString(plain, -1)
	if len(fields) == 0 {
		return 0, false, false
	}
	editable := true
	if isDeleted(raw) && !isAdded(raw) {
		editable = false
	}
	value := fields[len(fields)-1]
	if isAdded(raw) && !isDeleted(raw) {
		value = fields[0]
	}
	line, err := strconv.Atoi(value)
	if err != nil || line < 1 {
		return 0, false, false
	}
	return line, editable, true
}

func isAdded(s string) bool {
	return strings.Contains(s, "\x1b[32") || strings.Contains(s, "\x1b[92")
}

func isDeleted(s string) bool {
	return strings.Contains(s, "\x1b[31") || strings.Contains(s, "\x1b[91")
}
