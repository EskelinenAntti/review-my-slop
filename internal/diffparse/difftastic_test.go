package diffparse

import "testing"

func TestDifftasticParseMapsRenderedLinesToNewFileLocations(t *testing.T) {
	lines := []string{
		"\x1b[1mb/foo.go\x1b[0m --- Go",
		"\x1b[91m 8 removed\x1b[0m",
		"\x1b[92m 9 added\x1b[0m",
		"\x1b[91m 10 old\x1b[0m    \x1b[92m 11 new\x1b[0m",
	}

	parsed := (Difftastic{}).Parse(lines)

	if parsed[1].Location != (Location{File: "foo.go", Line: 8}) {
		t.Fatalf("deleted line location = %#v", parsed[1].Location)
	}
	if !parsed[1].Selectable || parsed[1].Editable {
		t.Fatalf("deleted line selectable/editable = %v/%v, want true/false", parsed[1].Selectable, parsed[1].Editable)
	}
	if parsed[2].Location != (Location{File: "foo.go", Line: 9}) {
		t.Fatalf("added line location = %#v", parsed[2].Location)
	}
	if !parsed[2].Selectable || !parsed[2].Editable {
		t.Fatalf("added line selectable/editable = %v/%v, want true/true", parsed[2].Selectable, parsed[2].Editable)
	}
	if parsed[3].Location != (Location{File: "foo.go", Line: 11}) {
		t.Fatalf("modified line location = %#v", parsed[3].Location)
	}
}

func TestDifftasticParsePreservesText(t *testing.T) {
	lines := []string{"a.go --- Go", "\x1b[92m 1 package main\x1b[0m"}

	parsed := (Difftastic{}).Parse(lines)

	for i := range lines {
		if parsed[i].Text != lines[i] {
			t.Fatalf("line %d text = %q, want %q", i, parsed[i].Text, lines[i])
		}
	}
}
