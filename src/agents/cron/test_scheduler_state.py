"""Storage format checks without a daemon or clock-driven runtime."""
import copy
from pathlib import Path
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import Mock
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'clients/python'))
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'shared'))
import scheduler


class KV:
    def __init__(self, values): self.values = copy.deepcopy(values); self.writes = []
    def list(self):
        return {kind: [{'name': key} for key, (k, _) in self.values.items() if k == kind] for kind in ('json', 'string')}
    def get(self, key, kind='string'): return copy.deepcopy(self.values[key][1])
    def set(self, key, value, **kwargs): self.writes.append(('set', key))
    def delete(self, key, **kwargs): self.writes.append(('delete', key))


class State(unittest.TestCase):
    key = '0123456789ab'
    row = {'cron': '* * * * *', 'target': '#target@test', 'body': 'work', 'by': '#caller@test', 'created': 1}
    def agent(self, values):
        kv = KV(values)
        agent = scheduler.Scheduler(SimpleNamespace(kv=kv), {}, log=lambda _: None)
        return agent, kv
    def test_current_rows_and_claims_are_loaded_without_rewrite(self):
        claim = f'fired.{self.key}.2026-10-28T12:00'
        a, kv = self.agent({'sub.'+self.key: ('json', self.row), claim: ('string', 'sent')})
        a.load()
        self.assertIn(self.key, a.subs)
        self.assertEqual(a.claims[claim], 'sent')
        self.assertEqual(kv.writes, [])
    def test_current_sending_claim_still_enters_unknown_recovery(self):
        claim = f'fired.{self.key}.2026-10-28T12:00'
        a, _ = self.agent({'sub.'+self.key: ('json', self.row), claim: ('string', 'sending')})
        a.drop = Mock()
        a.load()
        a.drop.assert_called_once_with(self.key, claim, 'send_unknown')
    def test_noncanonical_state_refuses_before_any_recovery_or_write(self):
        claim = f'fired.{self.key}.2026-10-28T12:00'
        cases = [
            {'sub.old-id': ('json', self.row)},
            {'sub.'+self.key: ('json', {**self.row, 'paused': 'denied'})},
            {'sub.'+self.key: ('json', {**self.row, 'migrated_from': 'old-id'})},
        ] + [{claim: ('string', value)} for value in ('unconfirmed', 'denied', 'gone', 'retrying 1 10', 'refused')]
        cases += [{'fired.old-id.once': ('string', 'sent')}]
        for bad in cases:
            with self.subTest(bad=bad):
                values = {'sub.'+self.key: ('json', self.row), claim: ('string', 'sending'), **bad}
                a, kv = self.agent(values)
                a.sweep = Mock()
                with self.assertRaisesRegex(ValueError, 'unsupported scheduler state'):
                    a.load()
                a.sweep.assert_not_called()
                self.assertEqual(kv.writes, [])
                self.assertEqual(a.subs, {})
                self.assertEqual(a.claims, {})
