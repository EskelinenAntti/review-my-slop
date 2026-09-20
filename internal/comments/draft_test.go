package comments

import (
	"strings"
	"testing"
)

func TestDraftRoundTripRemovesUnchangedSuggestion(t *testing.T) {
	anchor := Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := Draft("body", anchor)

	if got := StripUnchangedSuggestion(draft, anchor); got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}

func TestStripUnchangedSuggestionPreservesEditedSuggestion(t *testing.T) {
	anchor := Anchor{QuotedLines: []string{"-old()", "+new()"}}
	body := "comment\n\n```suggestion\nbetter()\n```\n"

	if got := StripUnchangedSuggestion(body, anchor); got != body {
		t.Fatalf("edited suggestion = %q, want %q", got, body)
	}
}

func TestDraftUsesFenceLongerThanSuggestedCode(t *testing.T) {
	anchor := Anchor{QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"}}
	draft := Draft("explain this", anchor)

	if !strings.Contains(draft, "`````suggestion") {
		t.Fatalf("draft fence = %q", draft)
	}
	if got := StripUnchangedSuggestion(draft, anchor); got != "explain this" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}

func TestDraftIncludesOnlyTheNewVersionOfChangedLines(t *testing.T) {
	anchor := Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}

	if got := Draft("comment", anchor); got != "comment\n\n```suggestion\nunchanged()\nnew()\n```\n" {
		t.Fatalf("draft = %q", got)
	}
}
