#!/bin/sh
# Installs the pinned pymysql and psycopg 3 (LGPL-3.0, psycopg.org) wheels of requirements.txt into the package with their licenses; needs pip and the index, or AB_DB_WHEELS.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/db>}
. "$here/../package-tree.sh"
package_begin "$dest"
mkdir -p "$dest"
install -m 0755 "$here/db.py" "$dest/db.py"
install -m 0644 "$here/agent.json" "$here/env.dist" "$here/../../../LICENSE.md" "$dest/"
install -d -m 0755 "$dest/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$dest/licenses/NOTICE.md"

mkdir -p "$dest/support/agents/shared" "$dest/support/lib"
install -m 0644 "$here/../shared/metadata.py" "$here/../shared/provider_http.py" "$dest/support/agents/shared/"
install -m 0644 "$here/../VERSION" "$dest/support/agents/VERSION"
install -m 0644 "$here/../protocol.json" "$dest/support/agents/protocol.json"
# The drivers come from the pinned wheels in requirements.txt, never from the host's
# site-packages, whose metadata varies by machine (J67). AB_DB_WHEELS names a directory
# of those wheels for an offline build; AB_DB_REQUIREMENTS replaces the pin file (tests).
set -- --disable-pip-version-check --quiet --no-deps --no-compile --only-binary=:all: --require-hashes \
  --target "$dest/support/lib" -r "${AB_DB_REQUIREMENTS:-$here/requirements.txt}"
[ -z "${AB_DB_WHEELS:-}" ] || set -- --no-index --find-links "$AB_DB_WHEELS" "$@"
python3 -m pip install "$@"
find "$dest/support/lib" \( -name __pycache__ -o -name '*.pyc' -o -name RECORD -o -name INSTALLER -o -name direct_url.json -o -name REQUESTED \) -prune -exec rm -rf {} +
cp "$here/NOTICE" "$dest/NOTICE"
package_commit
