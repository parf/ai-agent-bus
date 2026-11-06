#!/usr/bin/env python3
"""Private helper: which relative paths match one subscription filter, bounded in CPU; no credentials, no network."""
import json
from pathlib import Path
import resource
import signal
import sys

HERE=Path(__file__).resolve().parent
SUPPORT=HERE/'support' if (HERE/'support').is_dir() else HERE.parents[1]
sys.path.insert(0,str(SUPPORT/'agents/shared'))
import subscription_filters as filters


def main():
    raw=sys.stdin.buffer.read(16*1024*1024+1)
    if len(raw)>16*1024*1024:return 78
    try:
        request=json.loads(raw);pattern=request['pattern'];paths=request['paths']
        if not isinstance(pattern,str) or not isinstance(paths,list):return 78
        compiled=filters.compile_filter(pattern)
    except Exception:return 78
    resource.setrlimit(resource.RLIMIT_CPU,(3,3))
    signal.signal(signal.SIGXCPU,lambda *_:sys.exit(79))
    out=[]
    for path in paths:
        try:out.append(bool(filters.matches(compiled,filters.text(path))))
        except filters.FilterError:out.append(False)
    print(json.dumps(out,separators=(',',':')))
    return 0


if __name__=='__main__':sys.exit(main())
