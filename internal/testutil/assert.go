// Package testutil provides assertions shared by the repository's tests.
package testutil

import "testing"

// RequireEqual stops the test when the actual and expected values differ.
func RequireEqual[T comparable](t *testing.T, got, want T, format string, args ...any) {
	t.Helper()
	if got != want {
		t.Fatalf(format, args...)
	}
}

// CheckEqual reports unequal values and lets the test continue.
func CheckEqual[T comparable](t *testing.T, got, want T, format string, args ...any) {
	t.Helper()
	if got != want {
		t.Errorf(format, args...)
	}
}

// NoError stops the test if an operation failed.
func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
