// Package main runs the terminal review client and exports pending feedback.
package main

import (
	"testing"
)

func requireEqual[T comparable](t *testing.T, got, want T,
	format string,
	args ...any,
) {
	t.Helper()
	if got != want {
		t.Fatalf(format, args...)
	}
}
