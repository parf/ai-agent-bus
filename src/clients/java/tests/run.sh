#!/bin/bash
# Compiles the Java client and its tests into the repository's tmp/, then runs
# them against a throwaway daemon (tests/daemon.sh). Needs a JDK 21+ with the
# jdk.compiler module (no javac binary needed) and jackson-core, which this
# fetches once from Maven Central into tmp/java-deps at a pinned version and
# SHA-256, or takes from JACKSON_CORE_JAR.
#   src/clients/java/tests/run.sh [section]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
java_dir=$(cd "$here/.." && pwd)
root=$(cd "$java_dir/../../.." && pwd)
mkdir -p "$root/tmp/java-deps"
jackson_version=2.22.3
jackson_sha256=8a501126a385b25841915d839508f8a66e2a0dbc8a6709d055ef3b3e852b094c
jar=${JACKSON_CORE_JAR:-$root/tmp/java-deps/jackson-core-$jackson_version.jar}
if [ ! -s "$jar" ]; then
  curl -fsS --max-time 60 -o "$jar.part" \
    "https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-core/$jackson_version/jackson-core-$jackson_version.jar"
  mv "$jar.part" "$jar"
fi
echo "$jackson_sha256  $jar" | sha256sum -c --quiet - || { echo "jackson-core does not match its pinned SHA-256" >&2; exit 2; }
out=$(mktemp -d "$root/tmp/java-client.XXXXXX")
trap 'rm -rf "$out"' EXIT
javac() { java -m jdk.compiler/com.sun.tools.javac.Main "$@"; }
# -source/-target, not --release: a JRE-based JDK may lack ct.sym, so the Java 21
# API surface is not enforced here (README § Build).
javac -source 21 -target 21 -Xlint:all,-options,-serial -Werror -cp "$jar" -d "$out/main" $(find "$java_dir/src/main" -name '*.java')
javac -source 21 -target 21 -Xlint:all,-options,-serial,-unchecked -cp "$out/main:$jar" -d "$out/test" $(find "$java_dir/src/test" -name '*.java')
# The shared protocol cases every client adapter reads.
export AB_CLIENT_CONFORMANCE_CASES=${AB_CLIENT_CONFORMANCE_CASES:-$root/src/testdata/client-conformance}
TLS=1 "$here/daemon.sh" java -cp "$out/main:$out/test:$jar" agentbus.BusTest "$@"
