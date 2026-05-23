package main

import "github.com/anttieskelinen/review-my-slop/internal/github"

type lineRef struct {
	Index   int
	File    string
	Line    int
	Side    string
	Content string
}

type changedLine struct {
	LineIndex int
	Ref       lineRef
	Left      *lineRef
	Right     *lineRef
	Split     int
}

type prContext = github.PR

type reviewRange struct {
	Start lineRef
	End   lineRef
}

type reviewDraft = github.Draft

type reviewContext struct {
	PR    *prContext
	Draft reviewDraft
}

type keyResult struct {
	Key string
	Err error
}

type diffResult struct {
	Source diffSource
	Refs   []lineRef
	Lines  []string
	Err    error
}

type diffSource string

const (
	sourceLocal  diffSource = "local"
	sourceBranch diffSource = "branch"
)

type terminalState struct {
	settings string
}

type reviewState struct {
	args            []string
	source          diffSource
	sourceArgs      []string
	localAvailable  bool
	branchAvailable bool
	pr              *prContext
	prChecking      bool
	draft           reviewDraft
	changedLines    []changedLine
	lines           []string
	cursor          int
	selectionAnchor *int
	top             int
	message         string
}
