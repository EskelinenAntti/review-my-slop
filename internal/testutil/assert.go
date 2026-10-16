// Package testutil provides assertions shared by the repository's tests.
package testutil

import "testing"

// RequireValuesEqual stops the test when the actual and expected values differ.
func RequireValuesEqual[
	T comparable,
](t *testing.T, got, want T, format string, args ...any) {
	t.Helper()
	if got != want {
		t.Fatalf(format, args...)
	}
}

// CheckValuesEqual reports unequal values and lets the test continue.
func CheckValuesEqual[
	T comparable,
](t *testing.T, got, want T, format string, args ...any) {
	t.Helper()
	if got != want {
		t.Errorf(format, args...)
	}
}

// RequireNoError stops the test if an operation failed.
func RequireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
