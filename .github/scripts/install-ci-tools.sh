#!/usr/bin/env bash

set -euo pipefail

readonly SHELLCHECK_VERSION=0.11.0
readonly ACTIONLINT_VERSION=1.7.12

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

sha256_file() {
    local output
    output=$(shasum -a 256 "$1")
    printf '%s\n' "${output%%[[:space:]]*}"
}

os=$(uname -s)
arch=$(uname -m)
case "$os/$arch" in
    Darwin/arm64)
        shellcheck_platform=darwin.aarch64
        shellcheck_sha256=339b930feb1ea764467013cc1f72d09cd6b869ebf1013296ba9055ab2ffbd26f
        actionlint_platform=darwin_arm64
        actionlint_sha256=aba9ced2dee8d27fecca3dc7feb1a7f9a52caefa1eb46f3271ea66b6e0e6953f
        ;;
    Darwin/x86_64)
        shellcheck_platform=darwin.x86_64
        shellcheck_sha256=c2c15e08df0e8fbc374c335b230a7ee958c313fa5714817a59aa59f1aa594f51
        actionlint_platform=darwin_amd64
        actionlint_sha256=5b44c3bc2255115c9b69e30efc0fecdf498fdb63c5d58e17084fd5f16324c644
        ;;
    *) fail "CI linters do not have a pinned archive for $os/$arch" ;;
esac

repository_root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)
tools_root=${CI_TOOLS_DIR:-$repository_root/.github/ci-tools}
[[ $tools_root == /* && $tools_root != / && $tools_root != *$'\n'* ]] || fail "refusing unsafe CI tools directory"
if [[ -e $tools_root && ( -L $tools_root || ! -d $tools_root ) ]]; then
    fail "CI tools root must be a real directory"
fi
if [[ -e $tools_root/bin && ( -L $tools_root/bin || ! -d $tools_root/bin ) ]]; then
    fail "CI tools bin path must be a real directory"
fi
mkdir -p "$tools_root/bin"
work=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/intent-sh-ci-tools.XXXXXX")
trap 'rm -rf -- "$work"' EXIT HUP INT TERM

shellcheck_archive=$work/shellcheck.tar.gz
shellcheck_url=https://github.com/koalaman/shellcheck/releases/download/v$SHELLCHECK_VERSION/shellcheck-v$SHELLCHECK_VERSION.$shellcheck_platform.tar.gz
curl --fail --location --retry 3 --silent --show-error --output "$shellcheck_archive" "$shellcheck_url"
[[ $(sha256_file "$shellcheck_archive") == "$shellcheck_sha256" ]] || fail "ShellCheck archive checksum mismatch"
mkdir "$work/shellcheck"
tar -xzf "$shellcheck_archive" -C "$work/shellcheck" --strip-components=1
[[ -f $work/shellcheck/shellcheck && ! -L $work/shellcheck/shellcheck ]] || fail "ShellCheck archive did not contain a regular binary"

actionlint_archive=$work/actionlint.tar.gz
actionlint_url=https://github.com/rhysd/actionlint/releases/download/v$ACTIONLINT_VERSION/actionlint_${ACTIONLINT_VERSION}_${actionlint_platform}.tar.gz
curl --fail --location --retry 3 --silent --show-error --output "$actionlint_archive" "$actionlint_url"
[[ $(sha256_file "$actionlint_archive") == "$actionlint_sha256" ]] || fail "actionlint archive checksum mismatch"
mkdir "$work/actionlint"
tar -xzf "$actionlint_archive" -C "$work/actionlint"
[[ -f $work/actionlint/actionlint && ! -L $work/actionlint/actionlint ]] || fail "actionlint archive did not contain a regular binary"

install -m 0755 "$work/shellcheck/shellcheck" "$tools_root/bin/shellcheck.new"
install -m 0755 "$work/actionlint/actionlint" "$tools_root/bin/actionlint.new"
mv "$tools_root/bin/shellcheck.new" "$tools_root/bin/shellcheck"
mv "$tools_root/bin/actionlint.new" "$tools_root/bin/actionlint"

"$tools_root/bin/shellcheck" --version | grep -F "version: $SHELLCHECK_VERSION" >/dev/null || fail "ShellCheck version verification failed"
"$tools_root/bin/actionlint" -version | grep -F "$ACTIONLINT_VERSION" >/dev/null || fail "actionlint version verification failed"

if [[ -n ${GITHUB_PATH-} ]]; then
    printf '%s\n' "$tools_root/bin" >> "$GITHUB_PATH"
fi
printf 'installed ShellCheck %s and actionlint %s\n' "$SHELLCHECK_VERSION" "$ACTIONLINT_VERSION"
