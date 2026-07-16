#!/usr/bin/env bash

set -euo pipefail

script_directory=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
spec=$script_directory/../ci/bash32-fixture.env
cache_root=${INTENT_SH_TEST_BASH32_CACHE:-${RUNNER_TEMP:-${TMPDIR:-/tmp}}/intent-sh-bash32}
archive=${INTENT_SH_TEST_BASH32_ARCHIVE:-$cache_root/bash-3.2.57.tar.gz}

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

spec_value() {
    local key=$1 value
    [[ $(grep -c "^${key}=" "$spec" || true) == 1 ]] || fail "Bash 3.2 fixture specification is invalid"
    value=$(sed -n "s/^${key}=//p" "$spec")
    [[ -n $value ]] || fail "Bash 3.2 fixture specification is empty"
    printf '%s\n' "$value"
}

sha256_file() {
    local output
    if command -v sha256sum >/dev/null 2>&1; then
        output=$(sha256sum "$1")
    else
        output=$(shasum -a 256 "$1")
    fi
    printf '%s\n' "${output%%[[:space:]]*}"
}

export_bash() {
    if [[ -n ${GITHUB_ENV-} ]]; then
        printf 'INTENT_SH_TEST_BASH32=%s\n' "$1" >> "$GITHUB_ENV"
    fi
    printf '%s\n' "$1"
}

[[ -f $spec && ! -L $spec ]] || fail "Bash 3.2 fixture specification must be regular"
schema=$(spec_value BASH32_FIXTURE_SCHEMA)
revision=$(spec_value BASH32_INSTALLER_REVISION)
version=$(spec_value BASH32_VERSION)
url=$(spec_value BASH32_SOURCE_URL)
source_sha256=$(spec_value BASH32_SOURCE_SHA256)
license=$(spec_value BASH32_LICENSE)
[[ $schema =~ ^[0-9]+$ && $revision =~ ^[0-9]+$ && $version == 3.2.57 ]] || fail "Bash 3.2 fixture metadata is invalid"
[[ $source_sha256 =~ ^[0-9a-f]{64}$ && $license == GPL-2.0-or-later ]] || fail "Bash 3.2 fixture integrity metadata is invalid"
[[ -n $cache_root && $cache_root != / && ! -L $cache_root ]] || fail "refusing unsafe Bash 3.2 cache root"

if [[ $(uname -s) == Darwin ]]; then
    system_bash=/bin/bash
    [[ -f $system_bash && ! -L $system_bash ]] || fail "macOS system Bash is unavailable"
    [[ $($system_bash -c 'printf "%s.%s" "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"') == 3.2 ]] || fail "macOS system Bash is not version 3.2"
    export_bash "$system_bash"
    exit 0
fi

mkdir -p "$cache_root"
fixture=$cache_root/fixture
binary=$fixture/bash
manifest=$fixture/manifest
cache_valid() {
    [[ -f $binary && ! -L $binary && -x $binary && -f $manifest && ! -L $manifest ]] || return 1
    [[ $(sed -n 's/^schema=//p' "$manifest") == "$schema" ]] || return 1
    [[ $(sed -n 's/^revision=//p' "$manifest") == "$revision" ]] || return 1
    [[ $(sed -n 's/^version=//p' "$manifest") == "$version" ]] || return 1
    [[ $(sed -n 's/^sourceSHA256=//p' "$manifest") == "$source_sha256" ]] || return 1
    local digest
    digest=$(sed -n 's/^binarySHA256=//p' "$manifest")
    [[ $digest =~ ^[0-9a-f]{64}$ && $(sha256_file "$binary") == "$digest" ]] || return 1
    [[ $($binary -c 'printf "%s.%s.%s" "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}" "${BASH_VERSINFO[2]}"') == "$version" ]]
}

if cache_valid; then
    export_bash "$binary"
    exit 0
fi

if [[ ! -f $archive || -L $archive || $(sha256_file "$archive") != "$source_sha256" ]]; then
    [[ -z ${INTENT_SH_TEST_BASH32_ARCHIVE-} ]] || fail "Bash 3.2 archive override failed checksum verification"
    temporary_archive=$(mktemp "$cache_root/.bash32-download.XXXXXX")
    curl --fail --location --retry 3 --silent --show-error --output "$temporary_archive" "$url"
    [[ $(sha256_file "$temporary_archive") == "$source_sha256" ]] || { rm -f "$temporary_archive"; fail "downloaded Bash 3.2 archive checksum mismatch"; }
    mv "$temporary_archive" "$archive"
fi

work=$(mktemp -d "$cache_root/.build.XXXXXX")
trap 'rm -rf -- "$work"' EXIT HUP INT TERM
mkdir "$work/source"
tar -xzf "$archive" -C "$work/source" --strip-components=1
[[ -f $work/source/configure && ! -L $work/source/configure ]] || fail "Bash 3.2 source archive is incomplete"
(
    cd "$work/source"
    ./configure --prefix="$work/install" --without-bash-malloc >"$work/configure.log" 2>&1
    make -j2 >"$work/build.log" 2>&1
    make install >"$work/install.log" 2>&1
) || fail "pinned Bash 3.2 build failed"
built=$work/install/bin/bash
[[ -f $built && ! -L $built && -x $built ]] || fail "Bash 3.2 build did not produce a regular executable"
[[ $($built -c 'printf "%s.%s.%s" "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}" "${BASH_VERSINFO[2]}"') == "$version" ]] || fail "built Bash version mismatch"
digest=$(sha256_file "$built")
mkdir "$work/publish"
install -m 0755 "$built" "$work/publish/bash"
cat > "$work/publish/manifest" <<EOF
schema=$schema
revision=$revision
version=$version
sourceSHA256=$source_sha256
license=$license
binarySHA256=$digest
EOF
previous=$cache_root/.fixture-previous.$$
had_previous=0
if [[ -e $fixture || -L $fixture ]]; then
    mv "$fixture" "$previous"
    had_previous=1
fi
if ! mv "$work/publish" "$fixture"; then
    if [[ $had_previous == 1 ]]; then
        mv "$previous" "$fixture"
    fi
    fail "could not atomically publish Bash 3.2 fixture"
fi
rm -rf -- "$previous"
cache_valid || fail "published Bash 3.2 fixture failed verification"
export_bash "$binary"
