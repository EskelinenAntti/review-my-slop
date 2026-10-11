package app

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

func requireType[T any](t *testing.T, value any) T {
	t.Helper()
	typed, ok := value.(T)
	if !ok {
		t.Fatalf("unexpected message type %T", value)
	}
	return typed
}
