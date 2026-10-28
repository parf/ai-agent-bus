#!/bin/sh
# Needs pymysql and psycopg 3 (LGPL-3.0, psycopg.org) installed for python3; they are copied into the package with their licenses.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/db>}
mkdir -p "$dest"
install -m 0755 "$here/db.py" "$dest/db.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"

mkdir -p "$dest/support/agents/shared" "$dest/support/internal/version" "$dest/support/lib"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/provider_http.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../../internal/version/VERSION" "$dest/support/internal/version/"
python3 - "$dest/support/lib" <<'PY'
import importlib.metadata as m, pathlib, shutil, sys
out = pathlib.Path(sys.argv[1])
for dist in ('pymysql', 'psycopg', 'psycopg-binary'):
    d = m.distribution(dist)
    for f in d.files:
        if f.parts[0].endswith('.dist-info') and f.name not in ('RECORD', 'INSTALLER', 'direct_url.json') or not f.parts[0].endswith('.dist-info'):
            if f.suffix == '.pyc' or '__pycache__' in f.parts:
                continue
            target = out / f.as_posix()
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(d.locate_file(f), target)
PY
cp "$here/NOTICE" "$dest/NOTICE"
