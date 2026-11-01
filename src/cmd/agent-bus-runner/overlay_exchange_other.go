//go:build !linux

package main

import "syscall"

// Shipped-file mounts use Linux bubblewrap; keep other runner builds portable.
func exchangeOverlayDirs(stage, target string) error { return syscall.EINVAL }
