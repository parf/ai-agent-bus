package bigfile

import "syscall"

// nofollow makes an open refuse a symbolic link in the last path element.
const nofollow = syscall.O_NOFOLLOW
