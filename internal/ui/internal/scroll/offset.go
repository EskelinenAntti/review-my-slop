// Package scroll provides bounded movement for list and viewport offsets.
package scroll

// Offset moves offset by distance within [0, limit], bounding the distance
// before adding it to avoid integer overflow.
// Callers supply 0 <= offset <= limit.
func Offset(offset, distance, limit int) int {
	return offset + max(-offset, min(distance, limit-offset))
}
