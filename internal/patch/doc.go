// Package patch retrieves the Patch under review from the current working
// directory. Git execution, repository discovery, parsing, and source reads
// stay behind Get. Returned snapshots own their data and are treated as read-only.
package patch
