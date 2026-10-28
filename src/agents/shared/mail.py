"""Bounded mail transport primitives; no account selection or retry policy."""
import datetime
import signal
import smtplib
import ssl
import time


class MailError(ValueError):
    pass


def tls_context(cafile=None):
    context = ssl.create_default_context(cafile=cafile)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    return context


def expired(_signum, _frame):
    raise TimeoutError()


def deadline(envelope):
    seconds = 60.0
    value = envelope.get('deadline')
    if value and not value.startswith('0001-'):
        seconds = min(seconds, datetime.datetime.fromisoformat(value.replace('Z', '+00:00')).timestamp() - time.time() - .1)
    if seconds <= 0:
        raise TimeoutError()
    old = signal.signal(signal.SIGALRM, expired)
    signal.setitimer(signal.ITIMER_REAL, seconds)
    return old, seconds


class SMTP(smtplib.SMTP):
    """Limit both each reply line and a whole multiline reply before joining it."""
    data_started = False
    acceptance = b''

    def getreply(self):
        lines, total = [], 0
        if self.file is None:
            self.file = self.sock.makefile('rb')
        while True:
            line = self.file.readline(4097)
            total += len(line)
            if not line or len(line) > 4096 or total > 65536:
                raise MailError('SMTP reply is closed or exceeds its byte limit')
            if len(line) < 4 or not line[:3].isdigit() or line[3:4] not in (b' ', b'-'):
                raise MailError('invalid SMTP reply')
            code = int(line[:3])
            lines.append(line[4:].strip(b' \t\r\n'))
            if line[3:4] == b' ':
                return code, b'\n'.join(lines)

    def data(self, message):
        self.data_started = True
        code, result = super().data(message)
        self.acceptance = result
        return code, result


class SMTP_TLS(SMTP):
    def __init__(self, *args, context, **kwargs):
        self.context = context
        super().__init__(*args, **kwargs)

    def _get_socket(self, host, port, timeout):
        sock = super()._get_socket(host, port, timeout)
        try:
            return self.context.wrap_socket(sock, server_hostname=host)
        except BaseException:
            sock.close()
            raise
