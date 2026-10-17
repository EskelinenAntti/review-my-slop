package comments

import (
	"testing"
)

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func requireEqual[T comparable](
	t *testing.T, got, want T, format string, args ...any,
) {
	t.Helper()
	if got != want {
		t.Fatalf(format, args...)
	}
}
