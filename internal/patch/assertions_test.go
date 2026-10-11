package patch

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

func checkEqual[T comparable](
	t *testing.T,
	got, want T,
	format string,
	args ...any,
) {
	t.Helper()
	if got != want {
		t.Errorf(format, args...)
	}
}
