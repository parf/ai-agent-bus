#!/bin/bash
# The image's OCI labels, checked statically (no build): the four labels exist, the version label comes
# from a build argument the build checks against the archive, and the acceptance and the README pass it.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
fails=0
check() { # check NAME GREP-PATTERN FILE
  if grep -qE -- "$2" "$here/$3"; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi
}
check "the title label" 'org\.opencontainers\.image\.title="agent-bus"' Containerfile
check "the version label comes from the build argument" 'org\.opencontainers\.image\.version="\$VERSION"' Containerfile
check "the source label" 'org\.opencontainers\.image\.source="https://github\.com/parf/ai-agent-bus"' Containerfile
check "the licenses label matches the package metadata" 'org\.opencontainers\.image\.licenses="LicenseRef-PolyForm-Strict-1\.0\.0"' Containerfile
check "the version is a build argument" '^ARG VERSION$' Containerfile
check "the build refuses a missing VERSION" '\[ -n "\$VERSION" \]' Containerfile
check "the build refuses an archive of another version" '\[ "\$version" = "\$VERSION" \]' Containerfile
check "the acceptance passes the version to the build" 'podman build .*--build-arg "VERSION=\$version"' acceptance.sh
check "the acceptance checks the built image's version label" 'org\.opencontainers\.image\.version' acceptance.sh
check "the README's build command passes the version from the VERSION file" 'build-arg VERSION=\$\(cat src/internal/version/VERSION\)' README.md
# The one version literal is the VERSION file: no version number appears in a label.
if grep -E 'image\.version="[0-9]' "$here/Containerfile" >/dev/null; then echo "FAIL a version literal in a label"; fails=$((fails + 1)); else echo "ok   no version literal in a label"; fi
# The licenses label is the license the root LICENSE.md names.
if head -1 "$here/../../LICENSE.md" | grep -q 'PolyForm Strict License 1\.0\.0'; then echo "ok   LICENSE.md is PolyForm Strict 1.0.0"; else echo "FAIL LICENSE.md is not PolyForm Strict 1.0.0"; fails=$((fails + 1)); fi
[ "$fails" = 0 ] && echo "labels: all checks passed" || { echo "labels: $fails failed"; exit 1; }
