"""inotify against a real directory: completed writes, moves, new directories, overflow, the watch limit, and the agent on top of it."""
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE));sys.path.insert(0,str(HERE.parent/'shared'));sys.path.insert(0,str(HERE.parents[1]/'clients/python'))
import scan
import settings as cfg
import watch
import test_filescan as base
import filescan as fs


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

    def test_a_removed_directory_is_reported_as_a_prefix(self):
        os.makedirs(Path(self.root)/'d');(Path(self.root)/'d/f.md').write_text('x');self.start()
        import shutil;shutil.rmtree(Path(self.root)/'d');self.assertTrue(eventually(lambda:self.seen('d/')))

    def test_excluded_directories_and_links_are_not_watched(self):
        os.makedirs(Path(self.root)/'.git');outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        os.symlink(outside,Path(self.root)/'link')
        self.start(excluded=lambda rel:rel.startswith('.'))
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


def json_path(body):
    import json
    return json.loads(body)['path']


if __name__=='__main__':unittest.main()
