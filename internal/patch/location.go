package patch

import "path/filepath"

// WorkingTreeLocation resolves a diff line to a path and line number for an
// editor. Deleted lines use their old number in the current file; deleted files
// and lines without a positive number have no working-tree location.
func (f File) WorkingTreeLocation(
	root string,
	line Line,
) (string, LineNumber, bool) {
	number := line.NewNumber
	if number == 0 {
		number = line.OldNumber
	}
	if f.NewPath == "" || f.NewPath == "/dev/null" || number < 1 {
		return "", 0, false
	}
	path := f.NewPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	return path, number, true
}
