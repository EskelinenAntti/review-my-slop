package diff

import "testing"

func TestFileIdentityDoesNotDependOnDisplayText(t *testing.T) {
	file := File{OldPath: "odd\nname.go", NewPath: "odd\nname.go"}
	if got := file.Path(); got != "odd\nname.go" {
		t.Fatalf("path = %q", got)
	}
	if SameFile(file, File{NewPath: "odd\nname.go"}) == false {
		t.Fatal("shared new path was not treated as the same file")
	}
}

func TestEmptyFilePathsAreNotAnIdentity(t *testing.T) {
	if SameFile(File{}, File{}) {
		t.Fatal("empty paths should not identify unrelated files")
	}
}

func TestLineNumberIsChosenBySemanticSide(t *testing.T) {
	line := Line{Kind: Addition, OldNumber: 0, NewNumber: 12}
	if line.Number(true) != 0 || line.Number(false) != 12 || !line.Changed() {
		t.Fatalf("line = %#v", line)
	}
}
