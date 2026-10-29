"""Golden SSRF policy, independent of version-specific ipaddress shortcuts."""
import socket
import sys
from pathlib import Path
import unittest

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
from policy import Refused, public_address, url, resolve


def answers(*values):
    return [(socket.AF_INET6 if ':' in value else socket.AF_INET,socket.SOCK_STREAM,socket.IPPROTO_TCP,'',(value,80,0,0) if ':' in value else (value,80)) for value in values]


class Policy(unittest.TestCase):
    def test_private_special_and_embedded_addresses(self):
        bad=['0.0.0.0','0.1.2.3','10.1.2.3','100.64.0.1','127.0.0.1','169.254.169.254','172.16.0.1','192.0.0.9','192.0.2.1','192.88.99.1','192.168.1.1','198.18.0.1','198.51.100.1','203.0.113.1','224.0.0.1','240.0.0.1','255.255.255.255','::','::1','::127.0.0.1','::ffff:127.0.0.1','::ffff:8.8.8.8','64:ff9b::808:808','64:ff9b:1::a00:1','100::1','2001::1','2001:db8::1','2002:808:808::1','fe80::1','fc00::1','ff02::1','2606:4700::5efe:7f00:1','2606:4700::200:5efe:808:808']
        for ip in bad:
            with self.subTest(ip=ip),self.assertRaises(Refused):public_address(ip)
        for ip in ('8.8.8.8','1.1.1.1','2606:4700:4700::1111','2001:4860:4860::8888'):
            self.assertEqual(str(public_address(ip)),ip)
    def test_numeric_spellings_and_local_aliases_refuse_before_dns(self):
        for host in ('2130706433','0x7f.1','0177.0.0.1','127.1','1.2.3','0','LOCALHOST.','x.localhost','x.local','[fe80::1%25eth0]'):
            with self.subTest(host=host),self.assertRaises(Refused):url('http://'+host+'/')
    def test_strict_authority_scheme_ports_and_controls(self):
        for raw in ('file:///etc/passwd','ftp://public.example/a','http://user@public.example/','http://a@127.0.0.1@evil/','http://evil.example\\@127.0.0.1/','http://public.example/\n','http://public.example:25/','http://public.example:6379/','http://public.example:22/','http://[::1','http://public.example/#secret'):
            with self.subTest(raw=raw),self.assertRaises(Refused):url(raw)
        for port in (80,443,8080,8443):self.assertEqual(url('http://public.example:'+str(port)+'/').port,port)
        self.assertEqual(url('https://BÜCHER.example:8443/a?q=x').authority,'xn--bcher-kva.example:8443')
    def test_every_dns_answer_is_checked_before_selecting_one(self):
        calls=[]
        def resolver(*args,**kwargs):calls.append((args,kwargs));return answers('8.8.8.8','127.0.0.1')
        with self.assertRaises(Refused):resolve(url('http://public.example'),resolver)
        self.assertEqual(len(calls),1)
        self.assertEqual(resolve(url('http://public.example'),lambda *a,**k:answers(*(['8.8.8.8']*15))),[(socket.AF_INET,'8.8.8.8')])
        with self.assertRaises(Refused):resolve(url('http://public.example'),lambda *a,**k:answers(*(['8.8.8.8']*17)))
    def test_literals_never_reach_dns_and_still_use_policy(self):
        def no_dns(*args,**kwargs):self.fail('literal reached DNS')
        self.assertEqual(resolve(url('https://8.8.8.8/'),no_dns),[(socket.AF_INET,'8.8.8.8')])
        for raw in ('http://127.0.0.1/','http://[::ffff:7f00:1]/','http://[64:ff9b::808:808]/'):
            with self.subTest(raw=raw),self.assertRaises(Refused):resolve(url(raw),no_dns)


if __name__=='__main__':unittest.main()
