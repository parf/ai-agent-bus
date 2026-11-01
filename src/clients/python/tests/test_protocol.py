import http.server
import json
import threading
import unittest
from agent_bus import Bus, BusError, Refusal

class Protocol(unittest.TestCase):
    def bus(self, identity, refusal=None):
        calls=[]
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self,*args): pass
            def do_GET(self):
                calls.append(self.path)
                code,value=(200,identity) if self.path=='/identity' else (200,{'you':'owner@test'})
                if self.path=='/ls' and refusal: code,value=refusal
                data=json.dumps(value).encode();self.send_response(code);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
        server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
        thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        self.addCleanup(server.server_close);self.addCleanup(thread.join,2);self.addCleanup(server.shutdown)
        bus=Bus(addr='http://127.0.0.1:'+str(server.server_port),token='fixture',timeout=2);self.addCleanup(bus.close)
        return bus,calls

    def test_handshake(self):
        for identity,error in [({'protocol':2,'protocol_min':2},'daemon too new'),({'protocol':1.5},'protocol'),({'protocol':-1},'protocol'),({'protocol':None},'protocol'),({'protocol':True},'protocol'),({'protocol':1,'protocol_min':2},'protocol_min')]:
            with self.subTest(identity=identity):
                bus,calls=self.bus(identity)
                with self.assertRaisesRegex(BusError,error):bus.status()
                self.assertEqual(calls,['/identity'],'application request after fatal protocol error')
        for identity,old in [({'protocol':1,'protocol_min':1},False),({'protocol':2,'protocol_min':1},False),({},True),({'protocol':0,'protocol_min':0},True)]:
            bus,calls=self.bus(identity)
            with self.assertLogs('agent_bus',level='WARNING') if old else self.assertNoLogs('agent_bus',level='WARNING'):
                self.assertEqual(bus.status()['you'],'owner@test')
            self.assertEqual(calls[0],'/identity')

    def test_kinds_and_old_operation(self):
        for identity in ({},{'protocol':1,'protocol_min':1}):
            for code,kind in [(404,'unknown'),(404,'no_route'),(503,'unavailable'),(500,None)]:
                bus,_=self.bus(identity,(code,{'error':'cause','kind':kind}))
                with self.assertRaises(Refusal) as caught:bus._request('GET','/ls')
                self.assertEqual(caught.exception.status,code);self.assertEqual(caught.exception.kind,kind)
                wanted='internal daemon error' if code==500 else 'daemon too old for GET /ls' if not identity and kind=='no_route' else 'cause'
                self.assertEqual(str(caught.exception),wanted)

if __name__=='__main__':unittest.main()
