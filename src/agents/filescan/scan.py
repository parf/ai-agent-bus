"""The poll walk and what a message carries; no symlink under `dir` is ever followed."""
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


def walk(root,include,exclude):
    """(kind, relative path, stat or errno) for every regular file under root, never through a link.

    Kind `file` carries the stat; `error` an errno for a file it could not stat; `direrror` one for a directory it could not read
    (its path may be empty for the root).
    Links, sockets, pipes and devices are skipped, and an excluded directory is not entered.
    """
    errors=[]
    try:top=os.open(root,OPEN_DIR)
    except OSError as error:
        yield ('error','',error.errno);return
    try:
        def onerror(error):errors.append(('direrror',(error.filename or '').lstrip('./') if isinstance(error.filename,str) else '',error.errno))
        for base,dirs,files,fd in os.fwalk(dir_fd=top,top='.',follow_symlinks=False,onerror=onerror):
            prefix='' if base=='.' else base[2:]+'/'
            keep=[]
            for d in sorted(dirs):
                try:link=stat.S_ISLNK(os.stat(d,dir_fd=fd,follow_symlinks=False).st_mode)
                except OSError:link=True
                if not link and not excluded(prefix+d,exclude):keep.append(d)
            dirs[:]=keep
            for name in sorted(files):
                rel=prefix+name
                if not selected(rel,include,exclude):continue
                try:st=os.stat(name,dir_fd=fd,follow_symlinks=False)
                except OSError as error:
                    yield ('error',rel,error.errno);continue
                if stat.S_ISREG(st.st_mode):yield ('file',rel,st)
    finally:os.close(top)
    yield from errors


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


def poll(items,root,settings,*,now,hasher=None):
    """Compare the tree with the saved items and say what changed; nothing is written.

    A file whose size and modification time match its item is not hashed; one that differs is, and an
    unchanged hash only refreshes the time (a touch sends nothing). A file younger than `settle` seconds is left
    for the next scan. At most `max_per_scan` creates and changes are taken; the rest, and deletes beyond that
    point, wait. Returns {'upserts','refresh','deletes','skipped','capped','seen'}.
    """
    hasher=hasher or digest
    plan={'upserts':{},'refresh':{},'deletes':[],'skipped':[],'capped':False,'seen':0,'unreadable_root':None}
    present=set();taken=0;blocked=[]
    for kind,rel,info in walk(root,settings['include'],settings['exclude']):
        if kind=='direrror' and rel=='' or kind=='error' and rel=='':
            plan['unreadable_root']=info;continue
        if kind=='direrror':
            blocked.append(rel+'/');plan['skipped'].append((rel,'unreadable'));continue
        if kind=='error':
            present.add(rel);plan['skipped'].append((rel,'unreadable'));continue  # a file we could not stat is not a deletion
        present.add(rel);plan['seen']+=1
        item=items.get(rel)
        if item and item.get('size')==info.st_size and item.get('mtime_ns')==info.st_mtime_ns:continue
        if settings['settle'] and now-info.st_mtime<settings['settle']:continue
        if taken>=settings['max_per_scan']:
            plan['capped']=True;continue
        try:sha,size,_=hasher(root,rel,0)
        except OSError:
            plan['skipped'].append((rel,'unreadable'));continue
        record={'fp':sha,'size':size,'mtime_ns':info.st_mtime_ns}
        if item and item['fp']==sha:plan['refresh'][rel]=record;continue
        plan['upserts'][rel]=record;taken+=1
    if plan['unreadable_root'] is None:
        plan['deletes']=sorted(k for k in items if k not in present and selected(k,settings['include'],settings['exclude']) and not any(k.startswith(b) for b in blocked))
    return plan


def apply(items,plan,*,allow_deletes=True):
    """Fold a plan into the items; a held plan's deletes stay out."""
    items.update(plan['refresh']);items.update(plan['upserts'])
    if allow_deletes:
        for key in plan['deletes']:items.pop(key,None)
