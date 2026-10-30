//go:build !cgo

package proctitle

// Without cgo the title cannot be set: the process keeps the one it started with.
func setTitle(string) {}
