import signal
import time
import threading
import unittest
from unittest.mock import patch

from budget import alarm_budget


class BudgetTests(unittest.TestCase):
    def test_inner_exception_is_not_swallowed(self):
        with self.assertRaisesRegex(RuntimeError, 'fixture operation failed'):
            with alarm_budget(1):
                raise RuntimeError('fixture operation failed')

    def test_inner_restores_elapsed_outer_time_and_handler(self):
        handler = lambda *_: None
        with patch('budget.signal.getsignal', return_value=handler), \
             patch('budget.signal.getitimer', return_value=(10.0, 0.0)), \
             patch('budget.time.monotonic', side_effect=[100.0, 103.0]), \
             patch('budget.signal.signal') as install, \
             patch('budget.signal.setitimer') as timer:
            with alarm_budget(0.05):
                pass
            self.assertEqual(timer.call_args_list[0].args, (signal.ITIMER_REAL, 0.05))
            self.assertEqual(timer.call_args_list[-1].args, (signal.ITIMER_REAL, 7.0))
            self.assertEqual(install.call_args_list[-1].args, (signal.SIGALRM, handler))

    def test_inner_never_extends_an_earlier_outer_deadline(self):
        with patch('budget.signal.getsignal', return_value=lambda *_: None), \
             patch('budget.signal.getitimer', return_value=(0.01, 0.0)), \
             patch('budget.time.monotonic', return_value=100.0), \
             patch('budget.signal.signal'), patch('budget.signal.setitimer') as timer:
            with alarm_budget(0.05):
                pass
            self.assertEqual(timer.call_args_list[0].args, (signal.ITIMER_REAL, 0.01))

    def test_expired_outer_is_not_disabled_during_unwind(self):
        with patch('budget.signal.getsignal', return_value=lambda *_: None), \
             patch('budget.signal.getitimer', return_value=(1.0, 0.0)), \
             patch('budget.time.monotonic', side_effect=[100.0, 102.0]), \
             patch('budget.signal.signal'), patch('budget.signal.setitimer') as timer:
            with self.assertRaisesRegex(ValueError, 'fixture'):
                with alarm_budget(0.05):
                    raise ValueError('fixture')
            self.assertGreater(timer.call_args_list[-1].args[1], 0)
            self.assertLess(timer.call_args_list[-1].args[1], 0.001)

    def test_real_inner_timeout_preserves_outer_alarm(self):
        previous = signal.getsignal(signal.SIGALRM)
        class OuterExpired(Exception):
            pass
        class InnerExpired(Exception):
            pass
        try:
            with alarm_budget(0.15, OuterExpired):
                with self.assertRaises(InnerExpired):
                    with alarm_budget(0.01, InnerExpired):
                        time.sleep(0.1)
                self.assertGreater(signal.getitimer(signal.ITIMER_REAL)[0], 0)
            self.assertEqual(signal.getitimer(signal.ITIMER_REAL)[0], 0)
            self.assertIs(signal.getsignal(signal.SIGALRM), previous)
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous)

    def test_real_outer_timeout_keeps_its_exception_inside_inner_scope(self):
        class OuterExpired(Exception):
            pass
        class InnerExpired(Exception):
            pass
        with self.assertRaises(OuterExpired):
            with alarm_budget(0.01, OuterExpired):
                with alarm_budget(0.1, InnerExpired):
                    time.sleep(0.2)

    def test_invalid_budgets_do_not_change_the_timer(self):
        with patch('budget.signal.setitimer') as timer:
            for value in (True, False, 0, -1, float('inf'), float('nan'), '1'):
                with self.assertRaises(ValueError):
                    with alarm_budget(value):
                        self.fail('invalid scope entered')
            timer.assert_not_called()

    def test_handler_is_restored_if_timer_installation_fails(self):
        handler = lambda *_: None
        with patch('budget.signal.getsignal', return_value=handler), \
             patch('budget.signal.getitimer', return_value=(0.0, 0.0)), \
             patch('budget.signal.signal') as install, \
             patch('budget.signal.setitimer', side_effect=[OSError('fixture'), None]):
            with self.assertRaisesRegex(OSError, 'fixture'):
                with alarm_budget(0.01):
                    self.fail('timer installation failed')
            self.assertEqual(install.call_args_list[-1].args, (signal.SIGALRM, handler))

    def test_unknown_foreign_handler_is_refused_before_mutation(self):
        with patch('budget.signal.getsignal', return_value=None), \
             patch('budget.signal.getitimer', return_value=(0.0, 0.0)), \
             patch('budget.signal.signal') as install:
            with self.assertRaises(ValueError):
                with alarm_budget(0.01):
                    self.fail('unknown handler accepted')
            install.assert_not_called()

    def test_thread_call_is_refused_without_changing_timer(self):
        failures = []
        def run():
            try:
                with alarm_budget(0.01):
                    failures.append('scope entered')
            except ValueError as error:
                failures.append(str(error))
        with patch('budget.signal.setitimer') as timer:
            worker = threading.Thread(target=run)
            worker.start()
            worker.join(1)
            self.assertFalse(worker.is_alive())
            self.assertEqual(failures, ['alarm budget requires the main thread'])
            timer.assert_not_called()

    def test_outer_exception_wins_when_inner_unwind_passes_outer_deadline(self):
        class OuterExpired(Exception):
            pass
        class InnerExpired(Exception):
            pass
        def outer(*_):
            raise OuterExpired()
        installed = []
        with patch('budget.signal.getsignal', return_value=outer), \
             patch('budget.signal.getitimer', return_value=(0.01, 0.0)), \
             patch('budget.time.monotonic', side_effect=[100.0, 100.02, 100.02]), \
             patch('budget.signal.signal', side_effect=lambda _, handler: installed.append(handler)), \
             patch('budget.signal.setitimer'):
            with self.assertRaises((OuterExpired, InnerExpired)) as caught:
                with alarm_budget(0.1, InnerExpired):
                    installed[0](signal.SIGALRM, None)
            self.assertIs(type(caught.exception), OuterExpired)


if __name__ == '__main__':
    unittest.main()
