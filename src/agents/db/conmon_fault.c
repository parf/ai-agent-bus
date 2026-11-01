/* Test-only: model conmon's memory.events ENOENT race, without deleting host cgroups. */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/syscall.h>
#include <unistd.h>

static int missing(const char *path) {
    if (path && strstr(path, "/memory.events")) {
        const char *marker = getenv("J74_FAULT_LOG");
        if (marker) {
            int fd = syscall(SYS_openat, AT_FDCWD, marker, O_WRONLY | O_CREAT | O_APPEND, 0600);
            if (fd >= 0) {
                syscall(SYS_write, fd, path, strlen(path));
                syscall(SYS_write, fd, "\n", 1);
                syscall(SYS_close, fd);
            }
        }
        errno = ENOENT;
        return 1;
    }
    return 0;
}

static mode_t mode_arg(int flags, va_list args) {
    return (flags & O_CREAT) || (flags & O_TMPFILE) == O_TMPFILE ? va_arg(args, int) : 0;
}

int open(const char *path, int flags, ...) {
    va_list args;
    va_start(args, flags);
    mode_t mode = mode_arg(flags, args);
    va_end(args);
    if (missing(path)) return -1;
    return syscall(SYS_openat, AT_FDCWD, path, flags, mode);
}

int open64(const char *path, int flags, ...) {
    va_list args;
    va_start(args, flags);
    mode_t mode = mode_arg(flags, args);
    va_end(args);
    if (missing(path)) return -1;
    return syscall(SYS_openat, AT_FDCWD, path, flags, mode);
}

int openat(int dir, const char *path, int flags, ...) {
    va_list args;
    va_start(args, flags);
    mode_t mode = mode_arg(flags, args);
    va_end(args);
    if (missing(path)) return -1;
    return syscall(SYS_openat, dir, path, flags, mode);
}

FILE *fopen(const char *path, const char *mode) {
    if (missing(path)) return NULL;
    FILE *(*real)(const char *, const char *) = dlsym(RTLD_NEXT, "fopen");
    return real(path, mode);
}

FILE *fopen64(const char *path, const char *mode) {
    if (missing(path)) return NULL;
    FILE *(*real)(const char *, const char *) = dlsym(RTLD_NEXT, "fopen64");
    return real(path, mode);
}
