package main

import "golang.org/x/sys/unix"

// termiosGet is the ioctl that reads a terminal's settings.
const termiosGet = unix.TIOCGETA
