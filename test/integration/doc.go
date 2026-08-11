// Package integration holds black-box and smoke tests for HiveMind that run
// against the exported API surface (and, for the shell smoke test, a live
// deployment). Keeping a non-test file here lets `go build ./...` treat the
// directory as a normal package. See test/README.md.
package integration
