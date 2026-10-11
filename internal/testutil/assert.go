// Package testutil supplies assertions shared by the repository's tests.
package testutil

import "testing"

// NoError stops the test when an operation returns an error.
func NoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Require stops the test when condition is false.
func Require(t testing.TB, condition bool, args ...any) {
	t.Helper()
	if !condition {
		t.Fatal(args...)
	}
}

// Requiref stops the test with a formatted diagnostic when condition is false.
func Requiref(t testing.TB, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, args...)
	}
}

// Assert records a failure when condition is false and continues the test.
func Assert(t testing.TB, condition bool, args ...any) {
	t.Helper()
	if !condition {
		t.Error(args...)
	}
}

// Assertf records a formatted failure when condition is false.
func Assertf(t testing.TB, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Errorf(format, args...)
	}
}
