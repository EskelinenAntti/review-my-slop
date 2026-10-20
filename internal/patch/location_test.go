package patch

import "testing"

func TestWorkingTreeLocation(t *testing.T) {
	tests := []struct {
		name   string
		file   File
		line   Line
		path   string
		number LineNumber
		ok     bool
	}{
		{
			name: "context uses new number",
			file: File{NewPath: "nested/main.go"},
			line: Line{OldNumber: 2, NewNumber: 5},
			path: "/repo with spaces/nested/main.go", number: 5, ok: true,
		},
		{
			name: "addition",
			file: File{NewPath: "new.go"},
			line: Line{Kind: Addition, NewNumber: 3},
			path: "/repo with spaces/new.go", number: 3, ok: true,
		},
		{
			name: "deleted line uses old number in renamed file",
			file: File{OldPath: "old.go", NewPath: "renamed.go"},
			line: Line{Kind: Deletion, OldNumber: 7},
			path: "/repo with spaces/renamed.go", number: 7, ok: true,
		},
		{
			name: "absolute path",
			file: File{NewPath: "/other/main.go"},
			line: Line{NewNumber: 1},
			path: "/other/main.go", number: 1, ok: true,
		},
		{
			name: "deleted file with empty new path",
			file: File{OldPath: "deleted.go"},
			line: Line{Kind: Deletion, OldNumber: 1},
		},
		{
			name: "deleted file with null new path",
			file: File{OldPath: "deleted.go", NewPath: "/dev/null"},
			line: Line{Kind: Deletion, OldNumber: 1},
		},
		{
			name: "missing line numbers",
			file: File{NewPath: "main.go"},
		},
		{
			name: "negative new number does not fall back",
			file: File{NewPath: "main.go"},
			line: Line{OldNumber: 1, NewNumber: -1},
		},
		{
			name: "negative old number",
			file: File{NewPath: "main.go"},
			line: Line{OldNumber: -1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, number, ok := tt.file.WorkingTreeLocation(
				"/repo with spaces", tt.line,
			)
			if path != tt.path || number != tt.number || ok != tt.ok {
				t.Fatalf("location = (%q, %d, %v), want (%q, %d, %v)",
					path, number, ok, tt.path, tt.number, tt.ok)
			}
		})
	}
}
