"""The poll walk and what a message carries; no symlink under `dir` is ever followed."""
import errno
import fnmatch
import hashlib
import os
import stat

CHUNK=1024*1024
OPEN_FILE=os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK|os.O_NOCTTY|os.O_CLOEXEC
OPEN_DIR=os.O_RDONLY|os.O_NOFOLLOW|os.O_DIRECTORY|os.O_CLOEXEC


def selected(rel,include,exclude):
    """Globs on the relative path; exclude wins. A pattern also matches any path below a matching directory."""
    def hit(patterns):
        parts=rel.split('/')
        return any(fnmatch.fnmatchcase(rel,p) or any(fnmatch.fnmatchcase('/'.join(parts[:i]),p) for i in range(1,len(parts))) for p in patterns)
    return hit(include) and not hit(exclude)


def excluded(rel,exclude):
    parts=rel.split('/')
    return any(fnmatch.fnmatchcase(rel,p) or any(fnmatch.fnmatchcase('/'.join(parts[:i]),p) for i in range(1,len(parts))) for p in exclude)


def walk(root,include,exclude,sub='',before=None):
    """(kind, relative path, stat or errno) for every regular file under root, never through a link.

    Kind `file` carries the stat; `error` an errno for a file it could not stat; `direrror` one for a directory it could
    not read, named by its full path relative to root (empty only when the whole root cannot be opened).
    Links, sockets, pipes and devices are skipped, and an excluded directory is not entered. With `sub` (a directory path
    relative to root) only that directory is walked, reached without following a link, and paths and the include and
    exclude globs are those relative to root, as in a whole walk. `before(directory, fd)` is called with each directory's path
    relative to root (the top is `sub`, '' for the root) and the descriptor the walker has pinned for it, just before that
    directory is listed, so a caller can start watching exactly that directory first: whatever appears later raises an event,
    and whatever appeared earlier is in the listing. A callback that raises leaves no descriptor open.
    """
    lead=sub+'/' if sub else ''
    try:top=open_dir(root,sub)
    except OSError as error:
        yield ('direrror',sub,error.errno);return
    # Open descriptors are only those along the current path, and each is on the stack before anything is yielded,
    # so closing the generator at any point closes them all, and a wide directory costs no more than a deep one.
    stack=[]
    def listing(fd):
        try:return sorted(os.scandir(fd),key=lambda e:e.name),None
        except OSError as error:return None,error.errno
    try:
        stack.append([top,iter(()),lead])   # on the stack before anything can raise, so every path below closes it
        if before:before(sub,top)
        entries,err=listing(top)
        if err is not None:
            stack.pop();os.close(top);yield ('direrror',sub,err);return
        stack[-1][1]=iter(entries)
        while stack:
            fd,it,prefix=stack[-1]
            entry=next(it,None)
            if entry is None:
                stack.pop();os.close(fd);continue
            rel=prefix+entry.name
            try:
                if entry.is_symlink():continue
                if entry.is_dir(follow_symlinks=False):
                    if excluded(rel,exclude):continue
                    try:child=os.open(entry.name,OPEN_DIR,dir_fd=fd)
                    except OSError as error:
                        yield ('direrror',rel,error.errno);continue
                    stack.append([child,iter(()),rel+'/'])   # registered before the callback, which may raise
                    if before:before(rel,child)
                    found,err=listing(child)
                    if err is not None:
                        stack.pop();os.close(child);yield ('direrror',rel,err);continue
                    stack[-1][1]=iter(found)
                    continue
                if not selected(rel,include,exclude):continue
                st=entry.stat(follow_symlinks=False)
            except OSError as error:
                yield ('error',rel,error.errno);continue
            if stat.S_ISREG(st.st_mode):yield ('file',rel,st)
    finally:
        for fd,_,_ in stack:os.close(fd)


def is_blocked(key,blocked):
    """Whether `key` lies under a directory that could not be read, named by its full relative path."""
    return any(key.startswith(b) for b in blocked)


def open_dir(root,sub=''):
    """A directory fd for root/sub, opened one component at a time without following a link."""
    fd=os.open(root,OPEN_DIR)
    try:
        for part in (sub.split('/') if sub else []):
            if part in ('','.','..'):raise OSError('bad relative path')
            nxt=os.open(part,OPEN_DIR,dir_fd=fd);os.close(fd);fd=nxt
    except BaseException:
        os.close(fd);raise
    return fd


def open_under(root,rel):
    """Open a regular file below root without following any link on the way; the caller closes the fd."""
    parts=rel.split('/')
    if not rel or rel.startswith('/') or '' in parts or '.' in parts or '..' in parts:raise OSError('bad relative path')
    fd=os.open(root,OPEN_DIR)
    try:
        for part in parts[:-1]:
            nxt=os.open(part,OPEN_DIR,dir_fd=fd);os.close(fd);fd=nxt
        out=os.open(parts[-1],OPEN_FILE,dir_fd=fd)
    finally:os.close(fd)
    try:
        if not stat.S_ISREG(os.fstat(out).st_mode):raise OSError('not a regular file')
    except BaseException:os.close(out);raise
    return out


def digest(root,rel,keep=0):
    """(sha256 hex, size, text or None): text only when the file is at most `keep` bytes and valid UTF-8 with no NUL."""
    fd=open_under(root,rel);h=hashlib.sha256();size=0;kept=bytearray()
    try:
        while True:
            chunk=os.read(fd,CHUNK)
            if not chunk:break
            h.update(chunk);size+=len(chunk)
            if size<=keep:kept.extend(chunk)
    finally:os.close(fd)
    text=None
    if size<=keep:
        try:
            text=bytes(kept).decode('utf-8')
            if '\0' in text:text=None
        except UnicodeDecodeError:text=None
    return h.hexdigest(),size,text


def stat_under(root,rel):
    """lstat of a path below root without following a link on the way; None when it is not there."""
    parts=rel.split('/')
    if not rel or '' in parts or '.' in parts or '..' in parts:raise OSError('bad relative path')
    try:fd=os.open(root,OPEN_DIR)
    except FileNotFoundError:return None
    try:
        for part in parts[:-1]:
            nxt=os.open(part,OPEN_DIR,dir_fd=fd);os.close(fd);fd=nxt
        return os.stat(parts[-1],dir_fd=fd,follow_symlinks=False)
    except (FileNotFoundError,NotADirectoryError):return None
    except OSError as error:
        if error.errno==errno.ELOOP:return None  # a link where a directory was: nothing to read through it
        raise
    finally:os.close(fd)


def _new_plan():
    return {'upserts':{},'refresh':{},'deletes':[],'skipped':[],'capped':False,'seen':0,'unreadable_root':None}


def _consider(items,root,settings,now,hasher,plan,rel,st,taken):
    """One present file against its item; returns the new count of creates and changes taken."""
    item=items.get(rel)
    if item and item.get('size')==st.st_size and item.get('mtime_ns')==st.st_mtime_ns:return taken
    if settings['settle'] and now-st.st_mtime<settings['settle']:return taken
    if taken>=settings['max_per_scan']:
        plan['capped']=True;return taken
    try:sha,size,_=hasher(root,rel,0)
    except OSError:
        plan['skipped'].append((rel,'unreadable'));return taken
    record={'fp':sha,'size':size,'mtime_ns':st.st_mtime_ns}
    if item and item.get('from'):record['from']=item['from']   # the rename this file came from stays until it is delivered
    if item and item['fp']==sha:
        plan['refresh'][rel]=record;return taken
    plan['upserts'][rel]=record
    return taken+1


def poll(items,root,settings,*,now,hasher=None):
    """Compare the tree with the saved items and say what changed; nothing is written.

    A file whose size and modification time match its item is not hashed; one that differs is, and an
    unchanged hash only refreshes the time (a touch sends nothing). A file younger than `settle` seconds is left
    for the next scan. At most `max_per_scan` creates and changes are taken; the rest, and deletes beyond that
    point, wait. Returns {'upserts','refresh','deletes','skipped','capped','seen','unreadable_root'}.
    """
    hasher=hasher or digest
    plan=_new_plan();present=set();taken=0;blocked=[]
    for kind,rel,info in walk(root,settings['include'],settings['exclude']):
        if kind=='direrror' and rel=='':
            plan['unreadable_root']=info;continue
        if kind=='direrror':
            blocked.append(rel+'/');plan['skipped'].append((rel,'unreadable'));continue
        if kind=='error':
            present.add(rel);plan['skipped'].append((rel,'unreadable'));continue  # a file we could not stat is not a deletion
        present.add(rel);plan['seen']+=1
        taken=_consider(items,root,settings,now,hasher,plan,rel,info,taken)
    if plan['unreadable_root'] is None:
        plan['deletes']=sorted(k for k in items if k not in present and selected(k,settings['include'],settings['exclude']) and not is_blocked(k,blocked))
    return plan


def poll_paths(items,root,settings,paths,*,now,hasher=None):
    """The same comparison for just these paths (from the watch): a file path, or a directory prefix ending in `/`.

    A directory that is gone deletes the items below it; one that appeared is walked. `seen` counts the files
    still present among the saved items plus new ones, so the guard can tell an emptied tree.
    """
    hasher=hasher or digest
    plan=_new_plan();taken=0;gone=set();present=set();blocked=[]
    for path in paths:
        if path.endswith('/'):
            sub=path[:-1]
            st=stat_under(root,sub) if sub else None
            below=[k for k in items if k.startswith(path)]
            if st is None or not stat.S_ISDIR(st.st_mode):
                gone.update(below);continue
            if excluded(sub,settings['exclude']):continue  # an excluded directory is neither read nor reported
            for kind,full,info in walk(root,settings['include'],settings['exclude'],sub):
                if kind=='direrror':
                    blocked.append(full+'/');plan['skipped'].append((full,'unreadable'));continue
                if kind=='error':
                    present.add(full);plan['skipped'].append((full,'unreadable'));continue
                if kind!='file':continue
                present.add(full)
                taken=_consider(items,root,settings,now,hasher,plan,full,info,taken)
            gone.update(k for k in below if k not in present and selected(k,settings['include'],settings['exclude']) and not is_blocked(k,blocked))
            continue
        if not selected(path,settings['include'],settings['exclude']):continue
        st=stat_under(root,path)
        if st is None or not stat.S_ISREG(st.st_mode):
            if path in items:gone.add(path)
            continue
        present.add(path);taken=_consider(items,root,settings,now,hasher,plan,path,st,taken)
    plan['deletes']=sorted(k for k in gone if k in items and k not in present)
    plan['seen']=len(set(items)-set(plan['deletes'])|set(plan['upserts']))
    return plan


def apply(items,plan,*,allow_deletes=True):
    """Fold a plan into the items; a held plan's deletes stay out."""
    items.update(plan['refresh']);items.update(plan['upserts'])
    if allow_deletes:
        for key in plan['deletes']:items.pop(key,None)
