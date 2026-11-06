"""inotify against a real directory: completed writes, moves, new directories, overflow, the watch limit, and the agent on top of it."""
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest
import unittest.mock

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'shared'));sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
import scan
import settings as cfg
import watch
import test_filescan as base
import filescan as fs
from test_filescan import RagBus


def eventually(check,seconds=3.0):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check():return True
        time.sleep(0.02)
    return check()


class WatcherTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=os.path.realpath(self.tmp.name)
        self.got=[];self.lock=threading.Lock()

    def start(self,**kw):
        def on(paths,overflow):
            with self.lock:self.got.append((paths,overflow))
        try:self.w=watch.Watcher(self.root,on,quiet=0.05,**kw)
        except watch.Unavailable:self.skipTest('inotify is not available here')
        self.w.start();self.addCleanup(self.w.stop)

    def seen(self,path):
        with self.lock:return any(path in paths for paths,_ in self.got)

    def test_a_finished_write_is_reported_a_bare_create_is_not(self):
        self.start();fd=os.open(Path(self.root)/'a.md',os.O_WRONLY|os.O_CREAT);time.sleep(0.2)
        self.assertFalse(self.seen('a.md'),'created but still open');os.write(fd,b'x');os.close(fd)
        self.assertTrue(eventually(lambda:self.seen('a.md')))

    def test_moves_and_deletes_are_reported_on_both_sides(self):
        (Path(self.root)/'a.md').write_text('x');self.start()
        os.rename(Path(self.root)/'a.md',Path(self.root)/'b.md')
        self.assertTrue(eventually(lambda:self.seen('a.md') and self.seen('b.md')))
        os.unlink(Path(self.root)/'b.md');self.assertTrue(eventually(lambda:self.seen('b.md')))

    def test_a_new_directory_is_watched_and_its_early_files_are_found(self):
        self.start();os.makedirs(Path(self.root)/'d/deep');(Path(self.root)/'d/deep/f.md').write_text('x')
        self.assertTrue(eventually(lambda:self.seen('d/deep/f.md')),self.got)
        (Path(self.root)/'d/deep/g.md').write_text('y');self.assertTrue(eventually(lambda:self.seen('d/deep/g.md')),'a file in the new directory later')

    def test_a_file_made_while_a_new_directory_gets_its_watch_is_still_reported(self):
        # J116: the directory was listed and only then watched, so a file made between the two was in neither the list nor
        # an event. The hook makes the file at the very moment the watch is being added, the worst moment for that order.
        self.start();real=self.w._watch;made=[]
        def hooked(rel,fd):
            if rel=='d/deep' and not made:
                made.append(1);(Path(self.root)/'d/deep/f.md').write_text('early')
            return real(rel,fd)
        self.w._watch=hooked
        os.makedirs(Path(self.root)/'d/deep')
        self.assertTrue(eventually(lambda:self.seen('d/deep/f.md')),self.got)
        self.assertEqual(made,[1],'the hook ran, so the watch was added after the file existed')

    def test_a_file_made_while_the_new_directory_itself_gets_its_watch_is_still_reported(self):
        self.start();real=self.w._watch;made=[]
        def hooked(rel,fd):
            if rel=='top' and not made:made.append(1);(Path(self.root)/'top/g.md').write_text('early')
            return real(rel,fd)
        self.w._watch=hooked
        os.makedirs(Path(self.root)/'top')
        self.assertTrue(eventually(lambda:self.seen('top/g.md')),self.got);self.assertEqual(made,[1])

    def test_a_parent_swapped_for_a_link_after_it_was_opened_cannot_redirect_a_watch(self):
        outside=Path(self.root).parent/(Path(self.root).name+'-outside');(outside/'b').mkdir(parents=True);self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        (Path(self.root)/'a/b').mkdir(parents=True)
        self.start();w=self.w;real=w._watch;swapped=[]
        def swap(rel,fd):
            if rel=='a/b' and not swapped:
                swapped.append(1);(Path(self.root)/'a').rename(Path(self.root)/'saved');(Path(self.root)/'a').symlink_to(outside,target_is_directory=True)
            return real(rel,fd)
        w._watch=swap;w.by_path.clear();w.add_tree('')   # walk the whole tree again, swapping a parent midway
        self.assertEqual(swapped,[1])
        info=Path('/proc/self/fdinfo/%d'%w.fd).read_text()
        self.assertNotIn('ino:%x '%(outside/'b').stat().st_ino,info,'a watch went onto the directory outside the root')
        self.assertIn('ino:%x '%(Path(self.root)/'saved'/'b').stat().st_ino,info,'it watched the inode that was opened')

    def test_a_deep_new_tree_is_listed_with_every_file_and_watched_in_every_directory(self):
        self.start();(Path(self.root)/'t/a/b').mkdir(parents=True);(Path(self.root)/'t/a/b/x.md').write_text('x');(Path(self.root)/'t/y.md').write_text('y')
        self.assertTrue(eventually(lambda:self.seen('t/a/b/x.md') and self.seen('t/y.md')),self.got)
        self.assertTrue({'t','t/a','t/a/b'}<=set(self.w.by_path),sorted(self.w.by_path))

    def test_a_removed_directory_is_reported_as_a_prefix(self):
        os.makedirs(Path(self.root)/'d');(Path(self.root)/'d/f.md').write_text('x');self.start()
        import shutil;shutil.rmtree(Path(self.root)/'d');self.assertTrue(eventually(lambda:self.seen('d/')))

    def test_excluded_directories_and_links_are_not_watched(self):
        os.makedirs(Path(self.root)/'.git');outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        os.symlink(outside,Path(self.root)/'link')
        self.start(exclude=['.*','*/.*'])
        self.assertEqual(sorted(self.w.by_path),[''])
        (Path(outside)/'x.md').write_text('x');(Path(self.root)/'.git/y').write_text('y');time.sleep(0.3);self.assertEqual(self.got,[])

    def test_the_watch_limit_leaves_the_rest_to_the_poll_and_says_so(self):
        for n in range(4):os.makedirs(Path(self.root)/f'd{n}')
        self.start(watch_limit=2)
        self.assertTrue(self.w.limit_hit);self.assertEqual(len(self.w.wd),2);self.assertEqual(self.w.unwatched,3)

    def test_an_overflow_is_reported_so_a_full_scan_runs(self):
        self.start();w=self.w;data=watch.HEADER.pack(-1,watch.IN_Q_OVERFLOW,0,0);dirty=set()
        self.assertTrue(w._parse(data,dirty))

    def test_the_watched_root_going_away_asks_for_a_full_scan(self):
        os.makedirs(Path(self.root)/'r');self.root=str(Path(self.root)/'r');self.start();os.rmdir(self.root)
        self.assertTrue(eventually(lambda:any(o for _,o in self.got)))


class PollPathsTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=os.path.realpath(self.tmp.name);self.s=cfg.parse(None)

    def put(self,rel,data='x'):
        p=Path(self.root)/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)

    def test_files_prefixes_and_deletes(self):
        items={};self.put('a.md','1');self.put('d/b.md','2');self.put('d/e/c.md','3')
        scan.apply(items,scan.poll(items,self.root,self.s,now=time.time()))
        self.put('a.md','1!');plan=scan.poll_paths(items,self.root,self.s,['a.md','nothing.md'],now=time.time())
        self.assertEqual((sorted(plan['upserts']),plan['deletes']),(['a.md'],[]))
        import shutil;shutil.rmtree(Path(self.root)/'d');plan=scan.poll_paths(items,self.root,self.s,['d/'],now=time.time())
        self.assertEqual(plan['deletes'],['d/b.md','d/e/c.md'])
        os.unlink(Path(self.root)/'a.md');plan=scan.poll_paths(items,self.root,self.s,['a.md'],now=time.time());self.assertEqual(plan['deletes'],['a.md'])

    def test_an_event_directly_on_an_unreadable_directory_withholds_every_delete_under_it(self):
        if os.geteuid()==0:self.skipTest('root reads everything')
        items={};self.put('one/blocked/a.md','a');self.put('one/ok.md','o');scan.apply(items,scan.poll(items,self.root,self.s,now=time.time()))
        d=Path(self.root)/'one'/'blocked';os.chmod(d,0);self.addCleanup(lambda:os.chmod(d,0o700))
        plan=scan.poll_paths(items,self.root,self.s,['one/blocked/'],now=time.time())
        self.assertEqual(plan['deletes'],[],'one/blocked/a.md still exists, it merely cannot be read');self.assertIn(('one/blocked','unreadable'),plan['skipped'])

    def test_an_unreadable_directory_blocks_only_its_own_subtree_not_same_named_ones(self):
        if os.geteuid()==0:self.skipTest('root reads everything')
        items={};self.put('a/blocked/x.md','x');self.put('b/blocked/y.md','y');scan.apply(items,scan.poll(items,self.root,self.s,now=time.time()))
        d=Path(self.root)/'a'/'blocked';os.chmod(d,0);self.addCleanup(lambda:os.chmod(d,0o700))
        os.unlink(Path(self.root)/'b/blocked/y.md')
        plan=scan.poll(items,self.root,self.s,now=time.time())
        self.assertEqual(plan['deletes'],['b/blocked/y.md'],'b/blocked is a different directory and its delete is real')
        plan=scan.poll_paths(items,self.root,self.s,['b/'],now=time.time());self.assertEqual(plan['deletes'],['b/blocked/y.md'])

    def test_an_excluded_event_directory_is_never_opened(self):
        self.put('private/key.txt','secret');opened=[];real=os.open
        def spy(path,*a,**k):opened.append(os.fsdecode(path));return real(path,*a,**k)
        s={**self.s,'exclude':['private/*','private']}
        with unittest.mock.patch.object(scan.os,'open',spy):plan=scan.poll_paths({},self.root,s,['private/'],now=time.time())
        self.assertEqual(plan['upserts'],{});self.assertNotIn('private',opened,'the excluded directory was entered, not merely filtered')
        self.assertFalse([o for o in opened if o.endswith('key.txt')])

    def test_directory_event_deletes_missing_immediate_child(self):
        items={};self.put('sub/a.md','a');self.put('sub/b.md','b');scan.apply(items,scan.poll(items,self.root,self.s,now=time.time()))
        os.unlink(Path(self.root)/'sub/a.md');os.unlink(Path(self.root)/'sub/b.md')
        plan=scan.poll_paths(items,self.root,self.s,['sub/'],now=time.time())
        self.assertEqual(plan['deletes'],['sub/a.md','sub/b.md'],'the directory exists but is empty')

    def test_a_new_directory_prefix_is_walked(self):
        items={};self.put('n/x.md');self.put('n/m/y.md');plan=scan.poll_paths(items,self.root,self.s,['n/'],now=time.time())
        self.assertEqual(sorted(plan['upserts']),['n/m/y.md','n/x.md'])

    def test_a_path_through_a_link_or_a_non_file_reads_nothing(self):
        outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside));Path(outside,'s.md').write_text('s')
        os.symlink(outside,Path(self.root)/'link');os.symlink(Path(outside)/'s.md',Path(self.root)/'f.md');os.mkfifo(Path(self.root)/'p.md')
        plan=scan.poll_paths({},self.root,self.s,['link/s.md','f.md','p.md','link/'],now=time.time());self.assertEqual(plan['upserts'],{})

    def test_an_excluded_path_is_ignored_and_a_tracked_deleted_one_never_resurfaces_from_a_prefix_mismatch(self):
        items={'keep/z.md':{'fp':'1','size':1,'mtime_ns':1}};self.put('.hidden','x')
        plan=scan.poll_paths(items,self.root,self.s,['.hidden'],now=time.time());self.assertEqual((plan['upserts'],plan['deletes']),({},[]))

    def test_a_directory_event_never_reads_what_the_exclude_globs_hold_back(self):
        self.put('private/key.txt','secret');self.put('pub/a.md','a')
        s={**self.s,'exclude':['private/*','private']}
        plan=scan.poll_paths({},self.root,s,['private/','pub/'],now=time.time())
        self.assertEqual(sorted(plan['upserts']),['pub/a.md'],'private/key.txt is excluded, whichever way it was named')

    def test_a_directory_event_does_not_delete_what_include_matches_by_its_full_path(self):
        self.put('docs/new/x/a.md','a');self.put('other/b.md','b')
        s={**self.s,'include':['docs/*']};items={}
        scan.apply(items,scan.poll(items,self.root,s,now=time.time()));self.assertEqual(sorted(items),['docs/new/x/a.md'])
        plan=scan.poll_paths(items,self.root,s,['docs/new/'],now=time.time())
        self.assertEqual((plan['deletes'],plan['upserts']),([],{}),'no false delete from matching globs against the path under the event directory')

    def test_a_directory_is_walked_through_no_link_even_one_swapped_in_midway(self):
        outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside));Path(outside,'s.md').write_text('s')
        self.put('d/ok.md','ok');os.symlink(outside,Path(self.root)/'d'/'ln')
        os.symlink(outside,Path(self.root)/'swapped')
        plan=scan.poll_paths({},self.root,self.s,['d/','swapped/'],now=time.time());self.assertEqual(sorted(plan['upserts']),['d/ok.md'])

    def test_the_guard_sees_an_emptied_tree_through_the_watch_path(self):
        items={f'f{i}.md':{'fp':str(i),'size':1,'mtime_ns':1} for i in range(4)}
        plan=scan.poll_paths(items,self.root,self.s,[f'f{i}.md' for i in range(4)],now=time.time())
        self.assertEqual((len(plan['deletes']),plan['seen']),(4,0))


class AgentWatchTests(base.Base):
    def setUp(self):
        super().setUp();self.addCleanup(lambda:self.agent.stop_watch())

    def watch_agent(self,config=None,**kw):
        self.agent=self.make(config or {'mode':'watch'})
        if not self.agent.start_watch(quiet=0.05,**kw):self.skipTest('inotify is not available here')

    def test_a_written_file_reaches_the_subscriber_within_a_second_without_a_poll(self):
        self.add();self.agent.tick();self.watch_agent();self.put('hot.md','fire')
        self.assertTrue(eventually(lambda:any(json_path(b)=='hot.md' for _,b,_ in self.bus.sent),1.0),self.bus.sent)

    def test_a_file_that_is_both_listed_and_raises_an_event_is_one_change(self):
        # J116: with the watch added before the listing, a file made right after the watch exists is in the listing and
        # also raises its own event. The subscriber must still get exactly one change for it.
        self.add();self.agent.tick();self.bus.sent.clear();self.watch_agent({'mode':'watch'});watcher=self.agent.watcher;real=watcher._watch;made=[]
        def hooked(rel,fd):
            real(rel,fd)
            if rel=='d/deep' and not made:made.append(1);(Path(self.root)/'d/deep/f.md').write_text('both')
        watcher._watch=hooked
        os.makedirs(Path(self.root)/'d/deep')
        self.assertTrue(eventually(lambda:any(json_path(b)=='d/deep/f.md' for _,b,_ in self.bus.sent),3.0),self.bus.sent)
        time.sleep(0.4);self.agent.tick()
        self.assertEqual([json_path(b) for _,b,_ in self.bus.sent].count('d/deep/f.md'),1,'listed and evented, yet one change')
        self.assertEqual(made,[1])

    def test_the_same_file_by_watch_and_by_poll_is_sent_once(self):
        self.add();self.agent.tick();self.watch_agent({'mode':'both'});self.put('once.md','x')
        self.assertTrue(eventually(lambda:len(self.bus.sent)>=1));time.sleep(0.2);self.agent.tick();self.agent.tick()
        self.assertEqual([json_path(b) for _,b,_ in self.bus.sent],['once.md'])

    def test_a_deletion_by_watch_is_judged_by_the_guard(self):
        for i in range(10):self.put(f'f{i}.md',str(i))
        self.add();self.agent.tick();self.bus.sent.clear();self.watch_agent()
        for i in range(5):os.unlink(Path(self.root)/f'f{i}.md')
        self.assertTrue(eventually(lambda:self.agent.status()['paused'] is not None))
        self.assertEqual(self.bus.sent,[]);self.assertEqual(self.agent.status()['tracked'],10)

    def test_status_names_the_mode_and_a_fallback(self):
        self.assertEqual(self.agent.status()['effective_mode'],'both' if False else 'poll' if self.agent.settings['mode']=='poll' else 'poll')
        self.watch_agent({'mode':'both'});s=self.agent.status();self.assertEqual((s['effective_mode'],s['fallback']),('both',None))
        self.agent.stop_watch();self.agent=self.make({'mode':'watch'})
        self.assertEqual(self.agent.status()['effective_mode'],'poll','not started: no watch yet');self.assertTrue(self.agent.needs_timer())

    def test_the_watch_limit_is_a_named_fallback_and_the_timer_is_needed(self):
        for n in range(4):os.makedirs(Path(self.root)/f'd{n}')
        self.watch_agent({'mode':'watch'},watch_limit=2);s=self.agent.status()
        self.assertIn('watch limit reached',s['fallback']);self.assertTrue(self.agent.needs_timer());self.assertEqual(s['effective_mode'],'both')

    def test_an_overflow_runs_a_full_scan_and_finds_what_the_events_missed(self):
        self.add();self.agent.tick();self.bus.sent.clear();self.put('missed.md','x')
        self.agent.on_events([],True);self.assertEqual([json_path(b) for _,b,_ in self.bus.sent],['missed.md'])
        self.put('named.md','y');self.bus.sent.clear();self.agent.on_events(['named.md'],False)
        self.assertEqual([json_path(b) for _,b,_ in self.bus.sent],['named.md'])
        self.put('other.md','z');self.bus.sent.clear();self.agent.on_events(['named.md'],False)
        self.assertEqual(self.bus.sent,[],'without an overflow only the named paths are looked at')

    def test_poll_mode_never_starts_a_watch(self):
        self.agent=self.make({'mode':'poll'});self.assertFalse(self.agent.start_watch());self.assertIsNone(self.agent.watcher);self.assertTrue(self.agent.needs_timer())

    def test_a_full_watch_needs_no_timer(self):
        self.watch_agent({'mode':'watch'});self.assertFalse(self.agent.needs_timer())


class RenameWatchTests(AgentWatchTests):
    """A rename inside the tree is ONE `renamed` change, paired by the kernel's cookie; every test counts the changes exactly."""
    def setUp(self):
        super().setUp();self.out=tempfile.TemporaryDirectory();self.addCleanup(self.out.cleanup);self.outside=os.path.realpath(self.out.name)

    def start(self,files,mode='watch',target_extra='',config=None):
        for i in range(12):self.put(f'fill{i}.md',str(i))   # enough files that one rename is not a mass delete
        for name,text in files.items():self.put(name,text)
        self.agent=self.make({'batch':500});self.add(extra=target_extra);self.agent.tick();self.bus.sent.clear()
        self.watch_agent(dict({'mode':mode,'batch':500},**(config or {})))

    def changes(self):
        import json
        return [(m['change'],m['path'],m.get('old_path')) for m in (json.loads(b) for _,b,_ in self.bus.sent)]

    def settle(self,count,seconds=3.0):
        self.assertTrue(eventually(lambda:len(self.bus.sent)>=count,seconds),self.changes());time.sleep(0.4)   # then no stray extra

    def r(self,old,new):os.rename(Path(self.root)/old,Path(self.root)/new)

    def test_a_rename_in_one_directory_is_one_renamed_change(self):
        self.start({'a.md':'text'});self.r('a.md','b.md');self.settle(1)
        self.assertEqual(self.changes(),[('renamed','b.md','a.md')],'one change: no stray delete or create')
        import json;m=json.loads(self.bus.sent[0][1]);self.assertEqual((m['size'],m['text']),(4,'text'))
        marks=self.agent.state.load()['marks'][next(iter(self.agent.state.load()['subs']))]
        self.assertIn('b.md',marks);self.assertNotIn('a.md',marks)

    def test_a_rename_across_directories_is_one_renamed_change(self):
        self.start({'d1/a.md':'x','d2/keep.md':'k'});self.r('d1/a.md','d2/b.md');self.settle(1)
        self.assertEqual(self.changes(),[('renamed','d2/b.md','d1/a.md')])

    def test_a_directory_rename_is_one_renamed_change_per_file(self):
        self.start({'old/a.md':'1','old/sub/b.md':'2'});self.r('old','new');self.settle(2)
        self.assertEqual(sorted(self.changes()),[('renamed','new/a.md','old/a.md'),('renamed','new/sub/b.md','old/sub/b.md')])

    def test_renaming_many_files_at_once_is_not_a_mass_delete(self):
        self.start({f'big/f{i}.md':str(i) for i in range(30)});self.r('big','moved');self.settle(30,6.0)
        self.assertEqual(len([c for c in self.changes() if c[0]=='renamed']),30);self.assertEqual(len(self.changes()),30)
        self.assertIsNone(self.agent.status()['paused'],'files moved away are not deletes to the guard')

    def test_a_move_out_of_the_tree_is_a_delete_and_a_move_in_is_a_create(self):
        self.start({'a.md':'x'});os.rename(Path(self.root)/'a.md',Path(self.outside)/'a.md');self.settle(1)
        self.assertEqual(self.changes(),[('deleted','a.md',None)])
        self.bus.sent.clear();os.rename(Path(self.outside)/'a.md',Path(self.root)/'in.md');self.settle(1)
        self.assertEqual(self.changes(),[('created','in.md',None)])

    def test_an_unpaired_half_stays_a_delete_or_a_create(self):
        self.start({'a.md':'x'});self.agent.watcher._halves[7]='ghost.md'   # a from-half whose partner never came
        self.r('a.md','b.md');self.settle(1);self.assertEqual(self.changes(),[('renamed','b.md','a.md')])
        self.assertEqual(self.agent.watcher.take_moves(),{},'taken once')

    def test_a_write_right_after_the_rename_is_one_renamed_change_carrying_the_new_content(self):
        import hashlib,json
        self.start({'a.md':'old'});self.r('a.md','b.md');(Path(self.root)/'b.md').write_text('brand new');self.settle(1)
        self.assertEqual(self.changes(),[('renamed','b.md','a.md')]);m=json.loads(self.bus.sent[0][1])
        self.assertEqual((m['sha256'],m['text']),(hashlib.sha256(b'brand new').hexdigest(),'brand new'))

    def test_a_write_after_the_rename_was_delivered_is_a_changed(self):
        self.start({'a.md':'old'});self.r('a.md','b.md');self.settle(1);(Path(self.root)/'b.md').write_text('later');self.settle(2)
        self.assertEqual(self.changes(),[('renamed','b.md','a.md'),('changed','b.md',None)])

    def test_poll_mode_has_no_cookie_so_a_rename_is_a_delete_and_a_create(self):
        [self.put(f'fill{i}.md',str(i)) for i in range(12)];self.put('a.md','x')
        self.agent=self.make({'mode':'poll'});self.add();self.agent.tick();self.bus.sent.clear()
        self.r('a.md','b.md');self.agent.tick();self.assertEqual(sorted(self.changes()),[('created','b.md',None),('deleted','a.md',None)])

    def test_two_moves_in_a_row_reach_a_subscriber_that_never_got_the_first_as_one_rename_from_what_it_holds(self):
        [self.put(f'fill{i}.md',str(i)) for i in range(12)];self.put('a.md','text');self.agent=self.make({'batch':500})
        self.add();self.add('#late@n.test');self.agent.tick();self.bus.sent.clear();self.watch_agent({'mode':'watch','batch':500})
        self.bus.send_refused.add('#late@n.test');self.r('a.md','c.md')
        self.assertTrue(eventually(lambda:'c.md' in self.agent.state.load()['items'] and any(c==('renamed','c.md','a.md') for c in self.changes())))
        self.r('c.md','d.md');self.assertTrue(eventually(lambda:'d.md' in self.agent.state.load()['items']));time.sleep(0.4)
        self.bus.send_refused.clear();self.agent.tick();self.agent._round();time.sleep(0.2)
        by={}
        for to,b,_ in self.bus.sent:
            import json;m=json.loads(b);by.setdefault(to,[]).append((m['change'],m['path'],m.get('old_path')))
        self.assertEqual(by['#q@n.test'],[('renamed','c.md','a.md'),('renamed','d.md','c.md')],'the one that kept up saw both moves')
        self.assertEqual(by['#late@n.test'],[('renamed','d.md','a.md')],'it still held a: one rename from a, no stray delete or create')

    def test_the_origin_is_forgotten_once_the_rename_is_delivered_to_everyone_who_wants_it(self):
        self.start({'a.md':'x'});self.bus.send_refused.add('#q@n.test');self.r('a.md','b.md')
        item=lambda:self.agent.state.load()['items'].get('b.md',{})
        self.assertTrue(eventually(lambda:item().get('from')=='a.md'));time.sleep(0.3);self.assertEqual(item().get('from'),'a.md','not delivered yet: kept')
        self.bus.send_refused.clear();self.agent.tick();self.agent._round()
        self.assertEqual(self.changes(),[('renamed','b.md','a.md')]);self.assertNotIn('from',item(),'delivered: the chain is dropped')

    def test_a_delivered_rename_does_not_excuse_a_later_mass_delete(self):
        for i in range(12):self.put(f'fill{i}.md',str(i))
        for i in range(10):self.put(f'old/f{i}.md','v1 '+str(i))
        self.agent=self.make({'batch':500});self.add();self.agent.tick();self.bus.sent.clear()
        os.rename(Path(self.root)/'old',Path(self.root)/'new');self.agent.hints['new/']=('old/',self.agent.clock())   # what the cookie pairing hands over
        self.agent.tick();self.assertEqual([c[0] for c in self.changes()],['renamed']*10,'the move is delivered');self.assertIsNone(self.agent.status()['paused'])
        self.assertTrue(all('from' not in r for r in self.agent.state.load()['items'].values()),'nobody holds the old paths: the chains are gone')
        for i in range(10):self.put(f'old/f{i}.md','restored '+str(i))   # an unrelated old/ is made and delivered
        self.agent.tick();self.bus.sent.clear()
        for i in range(10):os.unlink(Path(self.root)/f'old/f{i}.md');self.put(f'new/f{i}.md','v2 '+str(i),mtime=time.time()+9)
        self.agent.tick();self.assertIsNotNone(self.agent.status()['paused'],'ten real deletes of 32 tracked hold the guard')

    def test_an_origin_is_kept_while_a_subscriber_may_still_hold_it(self):
        self.start({'a.md':'x'});self.r('a.md','b.md');self.settle(1)
        data=self.agent.state.load();sub_id=next(iter(data['subs']));data['items']['b.md']['from']='a.md';data['maybe'][sub_id]['a.md']='?';self.agent.state.save(data)
        self.bus.send_refused.add('#q@n.test');self.agent._round()   # the owed delete of a is refused, so the unknown send stays
        self.assertEqual(self.agent.state.load()['items']['b.md'].get('from'),'a.md','an unknown send of a: keep')
        self.bus.send_refused.clear();self.agent._round();self.assertNotIn('from',self.agent.state.load()['items']['b.md'])

    def test_a_changed_file_with_an_old_chain_does_not_exempt_its_recreated_origins_from_the_guard(self):
        for i in range(12):self.put(f'fill{i}.md',str(i))
        self.put('a.md','one');self.put('b.md','two');self.agent=self.make({'mode':'poll','batch':500});self.add();self.agent.tick();self.bus.sent.clear()
        data=self.agent.state.load();data['items']['b.md']['from']='a.md';self.agent.state.save(data)   # b came from a once, and a was made again since
        self.put('b.md','two, changed',mtime=time.time()+5);os.unlink(Path(self.root)/'a.md')
        for i in range(2):os.unlink(Path(self.root)/f'fill{i}.md')
        self.agent.tick();self.assertIsNotNone(self.agent.status()['paused'],'3 of 14 tracked is over 20 %: the old chain exempts none of them')
        self.assertEqual([c for c in self.changes() if c[0]=='deleted'],[])

    def test_a_filter_matching_one_side_sends_that_side_alone(self):
        self.start({'a.md':'x'},target_extra=r'/^a/');self.r('a.md','b.md');self.settle(1)
        self.assertEqual(self.changes(),[('deleted','a.md',None)],'only the old path matches')
        self.agent.stop_watch();self.bus.sent.clear();self.put('c.md','y');self.agent.tick();self.bus.sent.clear()
        self.agent=self.make({'mode':'watch'});self.add('#q2@n.test',extra=r'/^z/');self.agent.tick();self.bus.sent.clear()
        self.watch_agent({'mode':'watch'});self.r('c.md','z.md');self.settle(1)
        self.assertEqual([c for c in self.changes() if c[1]=='z.md'],[('created','z.md',None)],'only the new path matches')

    def test_a_rename_refused_by_the_target_is_sent_again_as_a_rename(self):
        self.start({'a.md':'x'});self.bus.send_refused.add('#q@n.test');self.r('a.md','b.md')
        self.assertTrue(eventually(lambda:self.agent.state.load()['items'].get('b.md',{}).get('from')=='a.md'));time.sleep(0.3)
        self.assertEqual(self.changes(),[]);self.bus.send_refused.clear();self.agent.tick();self.agent._round()
        self.assertEqual(self.changes(),[('renamed','b.md','a.md')],'the stored marks still held a.md, so it came back as the rename')

    def test_a_file_changed_or_touched_before_its_rename_is_delivered_is_still_the_one_rename(self):
        import json
        self.start({'a.md':'x'});self.bus.send_refused.add('#q@n.test');self.r('a.md','b.md')
        item=lambda:self.agent.state.load()['items'].get('b.md',{})
        self.assertTrue(eventually(lambda:item().get('from')=='a.md'))
        (Path(self.root)/'b.md').write_text('rewritten')
        self.assertTrue(eventually(lambda:item().get('size')==9));self.assertEqual(item().get('from'),'a.md','a change keeps the rename it came from')
        os.utime(Path(self.root)/'b.md',(1,1));self.agent.tick();self.assertEqual(item().get('from'),'a.md','a touch keeps it too')
        self.bus.send_refused.clear();self.agent.tick();self.agent._round()
        self.assertEqual(self.changes(),[('renamed','b.md','a.md')]);self.assertEqual(json.loads(self.bus.sent[0][1])['text'],'rewritten')

    def test_rag_has_no_rename_so_its_rows_keep_delete_then_upsert(self):
        self.bus=RagBus();self.agent=self.make({'mode':'watch'});self.put('a.md','text');self.bus.kinds[base.RAG]='agent'
        self.add(base.RAG,extra='as rag vault notes');self.agent.tick();self.bus.calls.clear()
        self.watch_agent({'mode':'watch'});self.r('a.md','b.md');self.assertTrue(eventually(lambda:len(self.bus.calls)>=2,3.0));time.sleep(0.4)
        import json;calls=[json.loads(b) for _,b,_ in self.bus.calls]
        self.assertEqual([(c['method'],c['id']) for c in calls],[('delete','a.md'),('upsert','b.md')])


def json_path(body):
    import json
    return json.loads(body)['path']


if __name__=='__main__':unittest.main()
