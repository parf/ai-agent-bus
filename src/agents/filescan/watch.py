"""Linux inotify through ctypes, so no package is added: one watch per directory, completed writes only.

It reports which relative paths to look at again and whether the kernel lost events; the agent decides what
changed by comparing them with its saved items. Nothing here is a guarantee: events are missed while stopped,
after an overflow, for changes other hosts make on a network filesystem, and past the watch limit.
"""
import ctypes
import ctypes.util
import errno
import os
import select
import struct
import threading
import time

IN_CLOSE_WRITE=0x8;IN_MOVED_FROM=0x40;IN_MOVED_TO=0x80;IN_CREATE=0x100;IN_DELETE=0x200;IN_DELETE_SELF=0x400;IN_MOVE_SELF=0x800
IN_Q_OVERFLOW=0x4000;IN_IGNORED=0x8000;IN_ISDIR=0x40000000;IN_ONLYDIR=0x1000000;IN_DONT_FOLLOW=0x2000000;IN_EXCL_UNLINK=0x4000000
IN_NONBLOCK=0o4000;IN_CLOEXEC=0o2000000
MASK=IN_CLOSE_WRITE|IN_MOVED_FROM|IN_MOVED_TO|IN_CREATE|IN_DELETE|IN_DELETE_SELF|IN_MOVE_SELF|IN_ONLYDIR|IN_DONT_FOLLOW|IN_EXCL_UNLINK
HEADER=struct.Struct('iIII')


class Unavailable(OSError):
    """inotify cannot be used here (not Linux, or the kernel refused); the agent polls instead."""


def _libc():
    name=ctypes.util.find_library('c')
    try:lib=ctypes.CDLL(name,use_errno=True)
    except (OSError,TypeError):raise Unavailable(errno.ENOSYS,'no C library') from None
    for fn in ('inotify_init1','inotify_add_watch','inotify_rm_watch'):
        if not hasattr(lib,fn):raise Unavailable(errno.ENOSYS,'no inotify')
    lib.inotify_init1.argtypes=[ctypes.c_int];lib.inotify_add_watch.argtypes=[ctypes.c_int,ctypes.c_char_p,ctypes.c_uint32]
    lib.inotify_rm_watch.argtypes=[ctypes.c_int,ctypes.c_int]
    return lib


class Watcher:
    """Watches `root` and every directory below it that it can; calls `on_events(paths, overflow)` when events settle.

    `paths` are relative file paths to look at again, plus directory prefixes ending in `/` for a directory that
    appeared or went away. `watch_limit` caps the watches this agent will hold (None: the kernel's limit); a
    directory beyond it is counted in `unwatched` and left to the poll.
    """

    def __init__(self,root,on_events,*,excluded=lambda rel:False,quiet=0.2,watch_limit=None):
        self.root=root;self.on_events=on_events;self.excluded=excluded;self.quiet=quiet;self.watch_limit=watch_limit
        self.lib=_libc();fd=self.lib.inotify_init1(IN_NONBLOCK|IN_CLOEXEC)
        if fd<0:
            e=ctypes.get_errno();raise Unavailable(e,os.strerror(e))
        self.fd=fd;self.wd={};self.by_path={};self.unwatched=0;self.limit_hit=False
        self._stop=threading.Event();self._thread=None;self._wake=os.pipe()
        for r in self._wake:os.set_blocking(r,False)

    def start(self):
        self.add_tree('')
        self._thread=threading.Thread(target=self._loop,name='filescan-watch',daemon=True);self._thread.start()

    def stop(self,timeout=5.0):
        self._stop.set()
        try:os.write(self._wake[1],b'x')
        except OSError:pass
        if self._thread:self._thread.join(timeout)
        done=not self._thread or not self._thread.is_alive()
        if done:
            for fd in (self.fd,*self._wake):
                try:os.close(fd)
                except OSError:pass
        return done

    def add_tree(self,rel):
        """Watch `rel` (a directory) and below, never through a link; return the files seen, as relative paths."""
        files=[];top=os.path.join(self.root,rel) if rel else self.root
        try:topfd=os.open(top,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        except OSError:return files
        try:
            for base,dirs,names,fd in os.fwalk('.',dir_fd=topfd,follow_symlinks=False):
                here=os.path.join(rel,base[2:]) if base!='.' else rel
                dirs[:]=sorted(d for d in dirs if not self.excluded((here+'/' if here else '')+d) and not os.path.islink(os.path.join(top,base,d)))
                self._watch(here)
                files.extend((here+'/' if here else '')+n for n in names)
        finally:os.close(topfd)
        return files

    def _watch(self,rel):
        if rel in self.by_path:return
        if self.watch_limit is not None and len(self.wd)>=self.watch_limit:
            self.unwatched+=1;self.limit_hit=True;return
        path=os.path.join(self.root,rel) if rel else self.root
        wd=self.lib.inotify_add_watch(self.fd,os.fsencode(path),MASK)
        if wd<0:
            e=ctypes.get_errno()
            if e==errno.ENOSPC:self.limit_hit=True
            self.unwatched+=1;return
        self.wd[wd]=rel;self.by_path[rel]=wd

    def _drop(self,rel):
        for path in [p for p in self.by_path if p==rel or p.startswith(rel+'/')]:
            wd=self.by_path.pop(path);self.wd.pop(wd,None)

    def _loop(self):
        dirty=set();overflow=False;last=None
        while not self._stop.is_set():
            timeout=None if last is None else max(0.0,last+self.quiet-time.monotonic())
            ready,_,_=select.select([self.fd,self._wake[0]],[],[],timeout)
            if self._stop.is_set():break
            if self.fd in ready:
                try:data=os.read(self.fd,65536)
                except BlockingIOError:data=b''
                except OSError:break
                o=self._parse(data,dirty);overflow=overflow or o
                if data:last=time.monotonic()
            if last is not None and time.monotonic()>=last+self.quiet:
                batch,over=sorted(dirty),overflow;dirty=set();overflow=False;last=None
                try:self.on_events(batch,over)
                except Exception:pass

    def _parse(self,data,dirty):
        overflow=False;i=0
        while i+HEADER.size<=len(data):
            wd,mask,cookie,length=HEADER.unpack_from(data,i);name=data[i+HEADER.size:i+HEADER.size+length].split(b'\0',1)[0];i+=HEADER.size+length
            if mask&IN_Q_OVERFLOW:overflow=True;continue
            rel=self.wd.get(wd)
            if rel is None:continue
            child=(rel+'/' if rel else '')+os.fsdecode(name) if name else rel
            if mask&IN_ISDIR and name:
                if self.excluded(child):continue
                if mask&(IN_CREATE|IN_MOVED_TO):
                    for f in self.add_tree(child):dirty.add(f)
                    dirty.add(child+'/')
                elif mask&(IN_DELETE|IN_MOVED_FROM):
                    self._drop(child);dirty.add(child+'/')
            elif mask&(IN_DELETE_SELF|IN_MOVE_SELF):
                if rel:self._drop(rel);dirty.add(rel+'/')
                else:overflow=True  # the watched root itself went away: let a full scan and the guard decide
            elif name and mask&(IN_CLOSE_WRITE|IN_MOVED_TO|IN_MOVED_FROM|IN_DELETE|IN_CREATE):
                if mask&IN_CREATE and not mask&(IN_MOVED_TO):continue  # only a finished write counts, not a bare create
                dirty.add(child)
        return overflow
