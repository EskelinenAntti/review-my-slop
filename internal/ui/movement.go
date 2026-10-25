package ui

// MoveWithinBounds applies delta and clamps the result to [0, maximum]. It
// bounds movement before addition to avoid integer overflow. The current
// position and maximum must be nonnegative.
func MoveWithinBounds(current, delta, maximum int) int {
	delta = max(-current, min(delta, maximum-current))
	return current + delta
}
