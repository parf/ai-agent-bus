#!/bin/sh
# agent-bus installer: curl -fsSL https://raw.githubusercontent.com/parf/ai-agent-bus/main/install.sh | sh
#
#   sh install.sh                 the full node: needs root (asks for sudo), then agent-bus-setup
#   sh install.sh --client        the client edition into ~/.local, no root
#   sh install.sh --version 0.9.x a given release instead of the latest; also the only way to go back
#   sh install.sh --key-fingerprint SHA256:...   refuse unless the built-in release key is that one
#   sh install.sh --client --rollback   switch the client edition back to its previous release
#   sh install.sh -- <setup args> everything after -- goes to agent-bus-setup (full node)
#
# Nothing is unpacked before the release's SHA256SUMS signature and the
# archive's hash are verified; a full node verifies again as root. Running it
# again updates, keeping configuration and data and the previous release for
# rollback. Docs: docs/07-setup.md#simple-install
#
# The whole script is functions, and main is called on its last line, so a
# download cut short runs nothing.

set -eu

RELEASES="https://github.com/parf/ai-agent-bus/releases"
NAMESPACE="agent-bus-release"
# The release signing keys, as ssh-keygen allowed_signers lines. Kept by the
# owner off this repository's machines; a second line carries the next key
# during a rotation.
# release-keys-begin
RELEASE_KEYS=""
# release-keys-end

say() { printf 'agent-bus install: %s\n' "$*" >&2; }
die() { say "$*"; exit 1; }

fetch() { # url file
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --https-only -O "$2" "$1"
    else
        die "needs curl or wget"
    fi
}

# Whether version $1 is older than $2.
older() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n 1)" = "$1" ]; }

# The client keeps two releases: current, and the one it replaced.
client_rollback() { # root bin
    current=$(readlink "$1/current" 2>/dev/null) || die "no client edition is installed in $1"
    current=$(basename "$current")
    previous=$(find "$1/releases" -mindepth 1 -maxdepth 1 -type d ! -name "$current" -printf '%f\n')
    [ -n "$previous" ] && [ "$(printf '%s\n' "$previous" | wc -l)" -eq 1 ] || die "no single previous release is kept in $1/releases"
    client_switch "$1" "$2" "$previous"
    say "rolled back from $current to $previous"
}

client_links="agent-bus agent-bus-runner agent-bus-mcp ab-claude ab-codex ab-opencode ab-opencode1"

# Refuse before anything is moved when a command name is a real file.
client_preflight() { # bin
    for p in $client_links; do
        if [ -e "$1/$p" ] && [ ! -L "$1/$p" ]; then
            die "$1/$p is not a link this installer made; move it and run again"
        fi
    done
}

client_switch() { # root bin release
    ln -sfn "releases/$3" "$1/current.new"
    mv -T "$1/current.new" "$1/current"
    mkdir -p "$2"
    for p in agent-bus agent-bus-runner; do ln -sfn "$1/current/$p" "$2/$p"; done
    # The Claude plugin starts the compiled MCP face by this name on PATH.
    ln -sfn "$1/current/agent-bus-ts" "$2/agent-bus-mcp"
    for p in ab-claude ab-codex ab-opencode ab-opencode1; do ln -sfn "$1/current/launchers/$p" "$2/$p"; done
}

# Run as root on a full node: copy the archive into a root-owned directory,
# check its signed hash and manifest again there, and only then run its
# setup. A user-owned tree is never executed by root. $1 archive, $2 hash,
# $3 name, the rest for setup.
# shellcheck disable=SC2016 # expanded by the root shell that runs it
ROOT_INSTALL='set -eu
archive=$1 want=$2 name=$3; shift 3
umask 022
mkdir -p /usr/local/lib
stage=$(mktemp -d /usr/local/lib/.agent-bus-install.XXXXXX)
trap "rm -rf \"$stage\"" EXIT INT TERM
cp "$archive" "$stage/release.tar.gz"
[ "$(sha256sum "$stage/release.tar.gz" | cut -d" " -f1)" = "$want" ] || { echo "agent-bus install: the archive changed after verification: nothing installed" >&2; exit 1; }
mkdir "$stage/x"
tar -xzf "$stage/release.tar.gz" -C "$stage/x" --no-same-owner
[ "$(ls -A "$stage/x")" = "$name" ] || { echo "agent-bus install: the archive does not hold exactly $name" >&2; exit 1; }
(cd "$stage/x/$name" && sha256sum -c --quiet MANIFEST.sha256) || { echo "agent-bus install: the archive does not match its own manifest" >&2; exit 1; }
"$stage/x/$name/agent-bus-setup" "$@"'

main() {
    edition=full version="" rollback="" fingerprint=""
    while [ $# -gt 0 ]; do
        case $1 in
            --client) edition=client ;;
            --version) [ $# -ge 2 ] || die "--version needs a version"; version=$2; shift ;;
            --version=*) version=${1#--version=} ;;
            --key-fingerprint) [ $# -ge 2 ] || die "--key-fingerprint needs SHA256:..."; fingerprint=$2; shift ;;
            --key-fingerprint=*) fingerprint=${1#--key-fingerprint=} ;;
            --rollback) rollback=1 ;;
            --) shift; break ;;
            -h|--help) sed -n '2,14p' "$0" 2>/dev/null || true; exit 0 ;;
            *) die "unknown option $1 (setup options go after --)" ;;
        esac
        shift
    done
    croot=$HOME/.local/lib/agent-bus cbin=$HOME/.local/bin
    if [ -n "$rollback" ]; then
        [ "$edition" = client ] || die "--rollback is for --client; a full node rolls back with agent-bus-setup"
        client_preflight "$cbin"
        client_rollback "$croot" "$cbin"
        return
    fi

    [ "$(uname -s)" = Linux ] || die "Linux only for now, not $(uname -s)"
    case $(uname -m) in
        x86_64|amd64) arch=x86_64 ;;
        aarch64|arm64) arch=aarch64 ;;
        *) die "no release for CPU $(uname -m)" ;;
    esac
    for tool in tar sha256sum ssh-keygen sort; do
        command -v "$tool" >/dev/null 2>&1 || die "needs $tool"
    done
    # ssh-keygen -Y arrived in OpenSSH 8.1; an older one cannot check a signature.
    if ssh-keygen -Y check-novalidate </dev/null 2>&1 | grep -qi 'unknown option\|illegal option'; then
        die "needs OpenSSH 8.1 or later (ssh-keygen -Y) to check the release signature"
    fi
    [ -n "$RELEASE_KEYS" ] || die "this install.sh carries no release key; use a published copy"
    case $version in
        "") ;;
        *[!0-9.]*|.*|*.|*..*) die "a version is MAJOR.MINOR.PATCH, not $version" ;;
    esac

    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT INT TERM
    printf '%s\n' "$RELEASE_KEYS" >"$work/allowed_signers"
    keyprints=$(printf '%s\n' "$RELEASE_KEYS" | while read -r _principal type blob _rest; do
        printf '%s %s\n' "$type" "$blob" >"$work/key.pub"
        ssh-keygen -l -f "$work/key.pub" | cut -d' ' -f2
    done)
    say "release key: $(printf '%s' "$keyprints" | tr '\n' ' ')"
    if [ -n "$fingerprint" ]; then
        printf '%s\n' "$keyprints" | grep -qxF "$fingerprint" || die "the built-in release key is not $fingerprint: nothing installed"
    fi

    if [ -n "$version" ]; then base="$RELEASES/download/v$version"; else base="$RELEASES/latest/download"; fi
    fetch "$base/SHA256SUMS" "$work/SHA256SUMS" || die "cannot download $base/SHA256SUMS"
    fetch "$base/SHA256SUMS.sig" "$work/SHA256SUMS.sig" || die "cannot download the signature of SHA256SUMS"
    ssh-keygen -Y verify -f "$work/allowed_signers" -I "$NAMESPACE" -n "$NAMESPACE" \
        -s "$work/SHA256SUMS.sig" <"$work/SHA256SUMS" >/dev/null 2>&1 ||
        die "SHA256SUMS is not signed by a release key: nothing installed"

    prefix=agent-bus
    [ "$edition" = client ] && prefix=agent-bus-client
    # Only lines of the exact form "<64 hex>  <prefix>-<version>-linux-<arch>.tar.gz";
    # the version is signed through that name.
    line=$(grep -E "^[0-9a-f]{64}  $prefix-[0-9]+\.[0-9]+\.[0-9]+-linux-$arch\.tar\.gz\$" "$work/SHA256SUMS" || true)
    [ -n "$line" ] && [ "$(printf '%s\n' "$line" | wc -l)" -eq 1 ] || die "the signed list names no single $prefix archive for linux-$arch"
    want=${line%% *}
    archive=${line##* }
    name=${archive%.tar.gz}
    release=${name#"$prefix"-}; release=${release%-linux-"$arch"}
    [ -z "$version" ] || [ "$release" = "$version" ] || die "the signed list is for $release, not $version"

    # An older signed release served as the latest is a downgrade nobody asked for.
    installed=""
    if [ "$edition" = client ]; then
        installed=$(basename "$(readlink "$croot/current" 2>/dev/null || true)")
    else
        current=$(readlink /usr/local/lib/agent-bus/current 2>/dev/null || true)
        [ -z "$current" ] || installed=$(cat "/usr/local/lib/agent-bus/$current/internal/version/VERSION" 2>/dev/null || true)
    fi
    if [ -n "$installed" ] && [ -z "$version" ] && older "$release" "$installed"; then
        die "the latest release is $release, older than the installed $installed: nothing installed (name a version with --version to go back)"
    fi
    if [ "$edition" = client ] && [ "$installed" = "$release" ]; then
        say "client edition $release is already current in $croot"
        return
    fi

    fetch "$base/$archive" "$work/$archive" || die "cannot download $archive"
    got=$(sha256sum "$work/$archive" | cut -d' ' -f1)
    [ "$got" = "$want" ] || die "$archive does not match its signed hash: nothing installed"

    if [ "$edition" = full ]; then
        upgrade=""
        [ -z "$installed" ] || upgrade=--upgrade
        if [ "$(id -u)" = 0 ]; then
            sh -c "$ROOT_INSTALL" sh "$work/$archive" "$want" "$name" $upgrade "$@"
        else
            command -v sudo >/dev/null 2>&1 || die "the full node needs root; run as root or install sudo, or use --client"
            say "the full node needs root: verifying again and running agent-bus-setup with sudo"
            sudo sh -c "$ROOT_INSTALL" sh "$work/$archive" "$want" "$name" $upgrade "$@"
        fi
        return
    fi

    mkdir "$work/x"
    tar -xzf "$work/$archive" -C "$work/x" --no-same-owner
    [ "$(ls -A "$work/x")" = "$name" ] || die "$archive does not hold exactly $name"
    (cd "$work/x/$name" && sha256sum -c --quiet MANIFEST.sha256) || die "$archive does not match its own manifest"
    client_preflight "$cbin"
    mkdir -p "$croot/releases"
    rm -rf "$croot/releases/$release"
    mv "$work/x/$name" "$croot/releases/$release"
    client_switch "$croot" "$cbin" "$release"
    # Keep the release now current and the one it replaced.
    find "$croot/releases" -mindepth 1 -maxdepth 1 -type d ! -name "$release" ! -name "${installed:-none}" -exec rm -rf {} +
    say "client edition $release is in $croot; commands are linked into $cbin"
    case :$PATH: in *":$cbin:"*) ;; *) say "add $cbin to PATH" ;; esac
}

main "$@"
