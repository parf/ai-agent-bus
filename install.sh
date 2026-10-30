#!/bin/sh
# agent-bus installer: curl -fsSL https://raw.githubusercontent.com/parf/ai-agent-bus/main/install.sh | sh
#
#   sh install.sh                 the full node: needs root (asks for sudo), then agent-bus-setup
#   sh install.sh --client        the client edition into ~/.local, no root
#   sh install.sh --version 0.9.x a given release instead of the latest
#   sh install.sh --client --rollback   switch the client edition back to its previous release
#   sh install.sh -- <setup args> everything after -- goes to agent-bus-setup (full node)
#
# Nothing is unpacked before the release's SHA256SUMS signature and the
# archive's hash are verified. Running it again updates, keeping configuration
# and data and the previous release for rollback. Docs: docs/07-setup.md#simple-install
#
# The whole script is one function called on its last line, so a download cut
# short runs nothing.

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
        curl -fsSL --proto '=https' --tlsv1.2 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --https-only -O "$2" "$1"
    else
        die "needs curl or wget"
    fi
}

# The client keeps two releases: current, and the one it replaced.
client_rollback() { # root bin
    current=$(readlink "$1/current" 2>/dev/null) || die "no client edition is installed in $1"
    current=$(basename "$current")
    previous=$(find "$1/releases" -mindepth 1 -maxdepth 1 -type d ! -name "$current" -printf '%f\n')
    [ -n "$previous" ] && [ "$(printf '%s\n' "$previous" | wc -l)" -eq 1 ] || die "no single previous release is kept in $1/releases"
    client_switch "$1" "$2" "$previous"
    say "rolled back from $current to $previous"
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

main() {
    edition=full version="" rollback=""
    while [ $# -gt 0 ]; do
        case $1 in
            --client) edition=client ;;
            --version) [ $# -ge 2 ] || die "--version needs a version"; version=$2; shift ;;
            --version=*) version=${1#--version=} ;;
            --rollback) rollback=1 ;;
            --) shift; break ;;
            -h|--help) sed -n '2,13p' "$0" 2>/dev/null || true; exit 0 ;;
            *) die "unknown option $1 (setup options go after --)" ;;
        esac
        shift
    done
    croot=${AGENT_BUS_HOME:-$HOME/.local/lib/agent-bus} cbin=$HOME/.local/bin
    if [ -n "$rollback" ]; then
        [ "$edition" = client ] || die "--rollback is for --client; a full node rolls back with agent-bus-setup"
        client_rollback "$croot" "$cbin"
        return
    fi

    [ "$(uname -s)" = Linux ] || die "Linux only for now, not $(uname -s)"
    case $(uname -m) in
        x86_64|amd64) arch=x86_64 ;;
        aarch64|arm64) arch=aarch64 ;;
        *) die "no release for CPU $(uname -m)" ;;
    esac
    for tool in tar sha256sum ssh-keygen; do
        command -v "$tool" >/dev/null 2>&1 || die "needs $tool"
    done
    [ -n "$RELEASE_KEYS" ] || die "this install.sh carries no release key; use a published copy"
    case $version in
        ""|[0-9]*.[0-9]*.[0-9]*) ;;
        *) die "a version is MAJOR.MINOR.PATCH, not $version" ;;
    esac
    case $version in *[!0-9.]*) die "a version is MAJOR.MINOR.PATCH, not $version" ;; esac

    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT INT TERM
    if [ -n "$version" ]; then base="$RELEASES/download/v$version"; else base="$RELEASES/latest/download"; fi
    fetch "$base/SHA256SUMS" "$work/SHA256SUMS" || die "cannot download $base/SHA256SUMS"
    fetch "$base/SHA256SUMS.sig" "$work/SHA256SUMS.sig" || die "cannot download the signature of SHA256SUMS"
    printf '%s\n' "$RELEASE_KEYS" >"$work/allowed_signers"
    ssh-keygen -Y verify -f "$work/allowed_signers" -I "$NAMESPACE" -n "$NAMESPACE" \
        -s "$work/SHA256SUMS.sig" <"$work/SHA256SUMS" >/dev/null 2>&1 ||
        die "SHA256SUMS is not signed by a release key: nothing installed"

    prefix=agent-bus
    [ "$edition" = client ] && prefix=agent-bus-client
    # Only lines of the exact form "<64 hex>  <prefix>-<version>-linux-<arch>.tar.gz".
    line=$(grep -E "^[0-9a-f]{64}  $prefix-[0-9]+\.[0-9]+\.[0-9]+-linux-$arch\.tar\.gz\$" "$work/SHA256SUMS" || true)
    [ -n "$line" ] && [ "$(printf '%s\n' "$line" | wc -l)" -eq 1 ] || die "the signed list names no single $prefix archive for linux-$arch"
    want=${line%% *}
    archive=${line##* }
    name=${archive%.tar.gz}
    release=${name#"$prefix"-}; release=${release%-linux-"$arch"}
    [ -z "$version" ] || [ "$release" = "$version" ] || die "the signed list is for $release, not $version"

    fetch "$base/$archive" "$work/$archive" || die "cannot download $archive"
    got=$(sha256sum "$work/$archive" | cut -d' ' -f1)
    [ "$got" = "$want" ] || die "$archive does not match its signed hash: nothing installed"

    mkdir "$work/x"
    tar -xzf "$work/$archive" -C "$work/x" --no-same-owner
    [ "$(ls -A "$work/x")" = "$name" ] || die "$archive does not hold exactly $name"
    (cd "$work/x/$name" && sha256sum -c --quiet MANIFEST.sha256) || die "$archive does not match its own manifest"

    if [ "$edition" = client ]; then
        mkdir -p "$croot/releases"
        before=$(basename "$(readlink "$croot/current" 2>/dev/null || echo none)")
        [ "$before" != "$release" ] || die "client edition $release is already current in $croot"
        rm -rf "$croot/releases/$release"
        mv "$work/x/$name" "$croot/releases/$release"
        client_switch "$croot" "$cbin" "$release"
        # Keep the release now current and the one it replaced.
        find "$croot/releases" -mindepth 1 -maxdepth 1 -type d ! -name "$release" ! -name "$before" -exec rm -rf {} +
        say "client edition $release is in $croot; commands are linked into $cbin"
        case :$PATH: in *":$cbin:"*) ;; *) say "add $cbin to PATH" ;; esac
        return
    fi

    setup="$work/x/$name/agent-bus-setup"
    upgrade=""
    [ -e /usr/local/lib/agent-bus/current ] && upgrade=--upgrade
    if [ "$(id -u)" = 0 ]; then
        "$setup" $upgrade "$@"
    else
        command -v sudo >/dev/null 2>&1 || die "the full node needs root; run as root or install sudo, or use --client"
        say "the full node needs root: running agent-bus-setup with sudo"
        sudo "$setup" $upgrade "$@"
    fi
}

main "$@"
