#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
exec "$here/run.sh" --demo
