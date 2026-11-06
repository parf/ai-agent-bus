"""The walk, the hash, the poll comparison; no link under the directory is ever followed."""
import errno
import os
from pathlib import Path
import socket
import sys
import tempfile
import time
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import scan
import settings as cfg


def put(root,rel,data='x',mtime=None):
    p=Path(root)/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data if isinstance(data,bytes) else data.encode())
    if mtime:os.utime(p,(mtime,mtime))
    return p


class Tree(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=os.path.realpath(self.tmp.name)
        self.s=cfg.parse(None)

    def names(self,**over):
        s={**self.s,**over}
        return sorted(rel for kind,rel,_ in scan.walk(self.root,s['include'],s['exclude']) if kind=='file')

    def poll(self,items,now=None,**over):
        return scan.poll(items,self.root,{**self.s,**over},now=now or time.time())


class SelectionTests(Tree):
    def test_include_and_exclude_globs_exclude_wins_and_default_skips_dotfiles(self):
        put(self.root,'a.md');put(self.root,'b.txt');put(self.root,'sub/c.md');put(self.root,'.hidden');put(self.root,'sub/.d.md');put(self.root,'.git/x.md')
        self.assertEqual(self.names(),['a.md','b.txt','sub/c.md'])
        self.assertEqual(self.names(include=['*.md']),['a.md','sub/c.md'])
        self.assertEqual(self.names(include=['*.md'],exclude=['sub/*','.*','*/.*']),['a.md'])
        self.assertEqual(self.names(exclude=[]),['.git/x.md','.hidden','a.md','b.txt','sub/.d.md','sub/c.md'])

    def test_an_excluded_directory_is_not_entered(self):
        put(self.root,'skip/deep/f.md');put(self.root,'keep/f.md')
        self.assertEqual(self.names(exclude=['skip']),['keep/f.md'])


class LinkTests(Tree):
    def test_links_sockets_and_pipes_are_never_listed_or_entered(self):
        put(self.root,'real.md');outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        put(outside,'secret.md','secret')
        os.symlink(outside,Path(self.root)/'dirlink');os.symlink(Path(outside)/'secret.md',Path(self.root)/'filelink.md')
        os.symlink('/proc/self/environ',Path(self.root)/'environ.md')
        os.mkfifo(Path(self.root)/'pipe.md')
        sock=socket.socket(socket.AF_UNIX);sock.bind(str(Path(self.root)/'sock.md'));self.addCleanup(sock.close)
        self.assertEqual(self.names(),['real.md'])

    def test_a_file_is_opened_without_following_a_link_at_any_level(self):
        put(self.root,'ok.md','fine');outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        put(outside,'secret.md','secret');os.symlink(outside,Path(self.root)/'dirlink');os.symlink(Path(outside)/'secret.md',Path(self.root)/'filelink.md')
        os.symlink('/proc/self/environ',Path(self.root)/'environ.md');os.mkfifo(Path(self.root)/'pipe.md')
        self.assertEqual(scan.digest(self.root,'ok.md',100)[2],'fine')
        for rel in ('filelink.md','dirlink/secret.md','environ.md','pipe.md','../x','a/../ok.md','/etc/passwd','','a//b','./ok.md'):
            with self.assertRaises(OSError,msg=rel):scan.digest(self.root,rel,100)
        try:scan.open_under(self.root,'environ.md')
        except OSError as e:self.assertEqual(e.errno,errno.ELOOP)

    def test_a_link_swapped_in_after_the_walk_is_not_read(self):
        put(self.root,'f.md','one');outside=tempfile.mkdtemp(prefix='fs-out-');self.addCleanup(lambda:__import__('shutil').rmtree(outside))
        put(outside,'secret.md','secret')
        p=Path(self.root)/'f.md';p.unlink();os.symlink(Path(outside)/'secret.md',p)
        with self.assertRaises(OSError):scan.digest(self.root,'f.md',100)


class DigestTests(Tree):
    def test_text_inline_only_when_small_utf8_without_nul(self):
        put(self.root,'t.md','héllo');put(self.root,'big.md','x'*50);put(self.root,'bin.dat',b'\xff\xfe\x00');put(self.root,'nul.md',b'a\x00b')
        sha,size,text=scan.digest(self.root,'t.md',100);self.assertEqual((size,text),(6,'héllo'));self.assertEqual(len(sha),64)
        self.assertEqual(scan.digest(self.root,'big.md',10)[2],None);self.assertEqual(scan.digest(self.root,'big.md',50)[2],'x'*50)
        self.assertEqual(scan.digest(self.root,'bin.dat',100)[2],None);self.assertEqual(scan.digest(self.root,'nul.md',100)[2],None)
        self.assertEqual(scan.digest(self.root,'big.md',0)[1:],(50,None),'a size-and-hash answer carries no prefix of the file')

    def test_the_hash_is_of_the_whole_file_past_the_chunk_size(self):
        import hashlib
        data=os.urandom(scan.CHUNK*2+17);put(self.root,'b.bin',data)
        self.assertEqual(scan.digest(self.root,'b.bin',0)[0],hashlib.sha256(data).hexdigest())


class PollTests(Tree):
    def test_created_changed_deleted_and_a_touch_that_changes_nothing(self):
        items={};put(self.root,'a.md','one',1000);put(self.root,'b.md','two',1000)
        p=self.poll(items);self.assertEqual(sorted(p['upserts']),['a.md','b.md']);scan.apply(items,p)
        self.assertEqual(self.poll(items)['upserts'],{});self.assertEqual(self.poll(items)['deletes'],[])
        put(self.root,'a.md','one',2000)  # touched: same content, new time
        p=self.poll(items);self.assertEqual((p['upserts'],sorted(p['refresh'])),({},['a.md']));scan.apply(items,p)
        self.assertEqual(self.poll(items)['refresh'],{})
        put(self.root,'a.md','one!',3000);p=self.poll(items);self.assertEqual(sorted(p['upserts']),['a.md']);scan.apply(items,p)
        os.unlink(Path(self.root)/'b.md');p=self.poll(items);self.assertEqual(p['deletes'],['b.md']);scan.apply(items,p);self.assertEqual(sorted(items),['a.md'])

    def test_only_a_file_whose_size_or_time_differs_is_hashed(self):
        items={};put(self.root,'a.md','one',1000);put(self.root,'b.md','two',1000);scan.apply(items,self.poll(items))
        hashed=[]
        def spy(root,rel,keep):hashed.append(rel);return scan.digest(root,rel,keep)
        put(self.root,'b.md','twp',2000)
        scan.poll(items,self.root,self.s,now=time.time(),hasher=spy);self.assertEqual(hashed,['b.md'])

    def test_a_file_made_and_removed_between_two_scans_is_never_reported(self):
        items={};put(self.root,'a.md');scan.apply(items,self.poll(items));put(self.root,'gone.md');os.unlink(Path(self.root)/'gone.md')
        p=self.poll(items);self.assertEqual((p['upserts'],p['deletes']),({},[]))

    def test_at_most_max_per_scan_changes_are_taken_and_the_rest_wait(self):
        items={};[put(self.root,f'f{n}.md',str(n)) for n in range(5)]
        p=self.poll(items,max_per_scan=2);self.assertEqual((len(p['upserts']),p['capped']),(2,True));scan.apply(items,p)
        p=self.poll(items,max_per_scan=2);self.assertEqual(len(p['upserts']),2);scan.apply(items,p)
        p=self.poll(items,max_per_scan=2);self.assertEqual((len(p['upserts']),p['capped']),(1,False));scan.apply(items,p);self.assertEqual(len(items),5)

    def test_a_file_younger_than_settle_waits(self):
        now=10000.0;put(self.root,'new.md','x',now-1);put(self.root,'old.md','x',now-100)
        p=self.poll({},now=now,settle=5);self.assertEqual(sorted(p['upserts']),['old.md'])
        p=self.poll({},now=now+10,settle=5);self.assertEqual(sorted(p['upserts']),['new.md','old.md'])

    def test_an_unreadable_file_is_skipped_named_and_never_a_delete(self):
        if os.geteuid()==0:self.skipTest('root reads everything')
        items={};put(self.root,'a.md','one');p=put(self.root,'locked.md','two');scan.apply(items,self.poll(items))
        os.chmod(p,0);self.addCleanup(lambda:os.chmod(p,0o600))
        put(self.root,'a.md','one-changed')  # force a rehash of a.md; locked.md keeps its time
        os.utime(p,(1,1))
        plan=self.poll(items);self.assertIn(('locked.md','unreadable'),plan['skipped']);self.assertEqual(plan['deletes'],[])
        self.assertIn('locked.md',items)

    def test_an_unreadable_subdirectory_withholds_deletes_under_it(self):
        if os.geteuid()==0:self.skipTest('root reads everything')
        items={};put(self.root,'d/x.md');put(self.root,'y.md');scan.apply(items,self.poll(items))
        d=Path(self.root)/'d';os.chmod(d,0);self.addCleanup(lambda:os.chmod(d,0o700))
        plan=self.poll(items);self.assertEqual(plan['deletes'],[]);self.assertIn(('d','unreadable'),plan['skipped'])

    def test_unreadable_nested_directory_is_not_a_delete(self):
        if os.geteuid()==0:self.skipTest('root reads everything')
        items={};put(self.root,'sub/blocked/a.md');put(self.root,'sub/ok.md');scan.apply(items,self.poll(items))
        d=Path(self.root)/'sub'/'blocked';os.chmod(d,0);self.addCleanup(lambda:os.chmod(d,0o700))
        plan=self.poll(items);self.assertEqual(plan['deletes'],[],'sub/blocked/a.md must not be called deleted');self.assertTrue(plan['skipped'])
        plan=scan.poll_paths(items,self.root,self.s,['sub/'],now=time.time());self.assertEqual(plan['deletes'],[])

    def test_an_unreadable_root_is_reported_and_deletes_nothing(self):
        items={'a.md':{'fp':'x','size':1,'mtime_ns':1}}
        plan=scan.poll(items,self.root+'/missing',self.s,now=time.time())
        self.assertIsNotNone(plan['unreadable_root']);self.assertEqual(plan['deletes'],[])

    def test_a_held_plans_deletes_stay_out_of_the_items(self):
        items={};put(self.root,'a.md');scan.apply(items,self.poll(items));os.unlink(Path(self.root)/'a.md')
        plan=self.poll(items);scan.apply(items,plan,allow_deletes=False);self.assertIn('a.md',items)
        scan.apply(items,plan);self.assertNotIn('a.md',items)


class SettingsTests(unittest.TestCase):
    def test_defaults_follow_the_plan(self):
        s=cfg.parse(None)
        self.assertEqual((s['mode'],s['every'],s['batch'],s['max_file_size'],s['max_per_scan'],s['delete_pct'],s['delete_max'],s['state_max'],s['baseline']),
                         ('both',1800.0,20,32768,200,20,500,64*1024*1024,False))
        self.assertEqual(cfg.parse({'mode':'poll'})['every'],300.0)

    def test_values_are_strict_and_unknown_keys_are_refused_by_name(self):
        for bad in ({'nope':1},{'mode':'x'},{'every':'0s'},{'every':'soon'},{'batch':0},{'batch':True},{'max_per_scan':0},{'baseline':'yes'},{'delete_pct':101},
                    {'include':'*.md'},{'exclude':[1]},{'state_max':10},{'max_file_size':'big'},{'namespace':'bad name'},{'dir':3},{'settle':-1},{'every':float('nan')}):
            with self.assertRaises(cfg.SettingsError,msg=str(bad)):cfg.parse(bad)
        with self.assertRaises(cfg.SettingsError) as c:cfg.parse({'nope':1})
        self.assertEqual(c.exception.key,'nope')
        self.assertEqual(cfg.parse({'every':'5m','settle':'10s','max_file_size':'1MiB','state_max':'2MiB'})['every'],300.0)


if __name__=='__main__':unittest.main()
