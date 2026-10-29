"""Main-thread alarm scopes that never extend an enclosing operation deadline."""
import contextlib
import math
import signal
import time


@contextlib.contextmanager
def alarm_budget(seconds, error=TimeoutError):
    """Limit one operation, retaining the earlier outer deadline and handler.

    This is for the IMAP main thread only, not a hard guarantee about a C
    resolver that does not return on signals. Repeating external timers are
    deliberately unsupported: the agent uses one-shot operation deadlines.
    """
    if not isinstance(seconds, (int, float)) or not math.isfinite(seconds) or seconds <= 0:
        raise ValueError('alarm budget must be finite and positive')
    before = time.monotonic()
    previous = signal.getsignal(signal.SIGALRM)
    remaining, interval = signal.getitimer(signal.ITIMER_REAL)
    if interval or (remaining and not callable(previous)):
        raise ValueError('enclosing alarm must be a callable one-shot deadline')
    outer = before + remaining if remaining else None

    def expire(signum, frame):
        if outer is not None and time.monotonic() >= outer:
            return previous(signum, frame)
        raise error()

    signal.signal(signal.SIGALRM, expire)
    try:
        signal.setitimer(signal.ITIMER_REAL, min(seconds, remaining) if remaining else seconds)
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous)
        if outer is not None:
            # An expired outer alarm must still fire; zero would disable it.
            signal.setitimer(signal.ITIMER_REAL, max(outer - time.monotonic(), 0.000001))
