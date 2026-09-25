package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestScoreCountsCallWithoutCalleeSelector(t *testing.T) {
	score := scoreSources(t, `package review

import "fmt"

func printValue(value string) { fmt.Println(value) }
`)

	if score != 1 {
		t.Fatalf("score=%d, want one call", score)
	}
}

func TestScoreCountsChainedCallsWithoutCalleeSelectors(t *testing.T) {
	score := scoreSources(t, `package review

func call(client interface{ API() interface{ Call() } }) { client.API().Call() }
`)

	if score != 2 {
		t.Fatalf("score=%d, want two calls", score)
	}
}

func TestWrapperDoesNotBeatDirectCall(t *testing.T) {
	direct := scoreSources(t, `package review

import "fmt"

func run(message string) error { return fmt.Errorf(message) }
`)
	wrapped := scoreSources(t, `package review

import "fmt"

func formatError(message string) error { return fmt.Errorf(message) }
func run(message string) error { return formatError(message) }
`)

	if wrapped <= direct {
		t.Fatalf("wrapped score=%d, direct score=%d", wrapped, direct)
	}
}

func TestSetterIsNotCheaperThanAssignment(t *testing.T) {
	direct := scoreSources(t, `package review

type model struct{ mode int }

func setDirect(m *model) { m.mode = 1 }
`)
	setter := scoreSources(t, `package review

type model struct{ mode int }

func (m *model) setMode(mode int) { m.mode = mode }
func setWithMethod(m *model) { m.setMode(1) }
`)

	if setter <= direct {
		t.Fatalf("setter score=%d, direct score=%d", setter, direct)
	}
}

func scoreSources(t *testing.T, source string) int {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	return scoreFile(file)
}
