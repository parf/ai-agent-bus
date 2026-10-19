"""Unchanged cleanup from /rd/service/tail-ring-buffer/server."""
import re
import unicodedata

LOG_CLEANUP_RE = re.compile(
    r'(?:'
    r'\x1b\][^\x07]*(?:\x07|\x1b\\)'   # OSC: title/hyperlink/etc.
    r'|\x1b\[[0-?]*[ -/]*[@-~]'        # CSI: ANSI color/control
    r'|\x1b[@-Z\\-_]'                  # 2-byte ESC sequences
    r'|#033(?:\[[0-?]*[ -/]*[@-~])?'   # syslog-escaped ESC
    r'|\[(?:\d{1,3};)*\d{1,3}m'        # SGR fragments after ESC loss in syslog
    r')'
)


def clean_line(line):
    line = LOG_CLEANUP_RE.sub('', line.rstrip('\n'))
    out = []
    in_bad_run = False

    for ch in line:
        if ch in '\t\r\n' or not unicodedata.category(ch).startswith('C'):
            out.append(ch)
            in_bad_run = False
        elif not in_bad_run:
            out.append('...')
            in_bad_run = True

    return ''.join(out)

