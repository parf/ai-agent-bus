package bigfile

import "syscall"

// nofollow makes an open refuse a symbolic link in the last path element.
const nofollow = syscall.O_NOFOLLOW

// nonblock keeps an open from waiting on a FIFO.
const nonblock = syscall.O_NONBLOCK
