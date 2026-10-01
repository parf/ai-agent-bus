#!/bin/bash
# Compiles the Java client and this example's classes into $1, fetching
# jackson-core as the client's tests do (pinned version and SHA-256). Prints the classpath.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
out=$1
mkdir -p "$root/tmp/java-deps" "$out"
jar=${JACKSON_CORE_JAR:-$root/tmp/java-deps/jackson-core-2.22.3.jar}
if [ ! -s "$jar" ]; then
  curl -fsS --max-time 60 -o "$jar.part" https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-core/2.22.3/jackson-core-2.22.3.jar
  mv "$jar.part" "$jar"
fi
echo "8a501126a385b25841915d839508f8a66e2a0dbc8a6709d055ef3b3e852b094c  $jar" | sha256sum -c --quiet - >&2
java -m jdk.compiler/com.sun.tools.javac.Main -source 21 -target 21 -nowarn -cp "$jar" -d "$out" \
  $(find "$root/src/clients/java/src/main" "$here/java" -name '*.java') >&2
echo "$out:$jar"
