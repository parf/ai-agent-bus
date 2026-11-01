"""A bigfile put or get runs on its own total budget, not the Bus timeout (E1b)."""
import hashlib
import http.server
import threading
import time
import unittest
from unittest.mock import patch

from agent_bus import Bus, BusError
from agent_bus import storage

DATA = b'x'


class Slow(unittest.TestCase):
    def bus(self, delay):
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def reply(self, body, headers=()):
                data = body.encode() if isinstance(body, str) else body
                self.send_response(200)
                self.send_header('Content-Length', str(len(data)))
                for key, value in headers:
                    self.send_header(key, value)
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                if self.path == '/identity':
                    return self.reply('{"protocol":1,"protocol_min":1}')
                if self.path == '/status':
                    return self.reply('{"you":"owner@fixture"}')
                time.sleep(delay)
                self.reply(DATA, [('X-Bigfile-Sha256', hashlib.sha256(DATA).hexdigest()), ('X-Bigfile-Held', 'memory')])

            def do_PUT(self):
                self.rfile.read(int(self.headers['Content-Length']))
                time.sleep(delay)
                self.reply('{"size":1,"sha256":"%s","replaced":false}' % hashlib.sha256(DATA).hexdigest())

        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.01}, daemon=True)
        thread.start()
        self.addCleanup(thread.join, 2)
        self.addCleanup(server.shutdown)
        self.addCleanup(server.server_close)
        bus = Bus(addr='http://127.0.0.1:%d' % server.server_port, token='fixture', timeout=0.3)
        self.addCleanup(bus.close)
        return bus.record('owner@fixture').bigfile

    def test_put_and_get_outlast_the_bus_timeout_within_their_budget(self):
        big = self.bus(0.6)  # twice the Bus timeout
        try:
            put = big.put('k', DATA, ttl=60)
            got = big.get('k')[0]
        except BusError as error:
            self.fail('a transfer inside its budget was cut at the Bus timeout: %s' % error)
        self.assertEqual((put['size'], got), (1, DATA))

    def test_a_transfer_past_its_budget_still_ends(self):
        big = self.bus(0.8)
        with patch.object(storage, 'BIG_IDLE', 0.2), patch.object(storage, 'BIG_RATE', 1 << 30):
            with self.assertRaises(BusError):
                big.get('k')
            with self.assertRaises(BusError):
                big.put('k', DATA, ttl=60)

    def test_budget_is_the_stall_allowance_plus_the_time_at_the_slowest_rate(self):
        self.assertEqual(storage.big_budget(64 * 1024), 31)
        self.assertEqual(storage.big_budget(storage.MAX_BIG), 542)
        self.assertEqual(storage.big_budget(10 * storage.MAX_BIG), 542)


if __name__ == '__main__':
    unittest.main()
