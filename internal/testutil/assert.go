// Package testutil provides assertions shared by the repository's tests.
package testutil

import "testing"

// RequireEqual stops the test when the actual and expected values differ.
func RequireEqual[
	T comparable,
](t *testing.T, got, want T, message string) {
	t.Helper()
	if got != want {
		t.Fatal(message)
	}
}

// CheckEqual reports unequal values and lets the test continue.
func CheckEqual[
	T comparable,
](t *testing.T, got, want T, message string) {
	t.Helper()
	if got != want {
		t.Error(message)
	}
}

// NoError stops the test if an operation failed.
func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
