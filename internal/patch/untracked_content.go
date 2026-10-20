package patch

import "fmt"

func parseUntrackedContent(path string, content []byte) File {
	if containsBinaryData(content) {
		return untrackedFile(path, "untracked binary file")
	}
	return addedFile(path, visibleSource(string(content)))
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, len(sourceLines))
	for i, line := range sourceLines {
		lines[i] = Line{
			Kind:      Addition,
			Text:      line,
			NewNumber: LineNumber(i + 1),
		}
	}
	file := untrackedFile(path, "untracked file")
	file.NewSource = content
	file.Hunks = []Hunk{{
		Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
		Lines:  lines,
	}}
	return file
}

func untrackedFile(path string, metadata ...string) File {
	return File{
		NewPath:     path,
		DisplayPath: visibleText(path),
		Metadata:    metadata,
	}
}
