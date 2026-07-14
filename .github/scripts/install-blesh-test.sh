#!/usr/bin/env bash

set -euo pipefail

readonly BLESH_COMMIT=d69e4d549a1881a37300fe6b4a05478bd9157dfc
readonly BLESH_VERSION=0.4.0-nightly+d69e4d5
readonly BLESH_SOURCE_SHA256=db583d869ec5afef0e6bd23bd1af38ec3aa2cc3e6062f8aa499633522b005394
readonly BLESH_SOURCE_URL=https://github.com/akinomyoga/ble.sh/archive/d69e4d549a1881a37300fe6b4a05478bd9157dfc.tar.gz

cache_root=${INTENT_SH_TEST_BLESH_CACHE:-${RUNNER_TEMP:-${TMPDIR:-/tmp}}/intent-sh-blesh}
archive=${INTENT_SH_TEST_BLESH_ARCHIVE:-$cache_root/ble.sh-$BLESH_COMMIT.tar.gz}
source_root=$cache_root/source/ble.sh-$BLESH_COMMIT
script=$source_root/out/ble.sh

mkdir -p "$cache_root"
if [[ ! -f $archive ]]; then
    curl --fail --location --retry 3 --output "$archive" "$BLESH_SOURCE_URL"
fi

if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$archive")
else
    actual=$(shasum -a 256 "$archive")
fi
actual=${actual%%[[:space:]]*}
if [[ $actual != "$BLESH_SOURCE_SHA256" ]]; then
    printf 'ble.sh source checksum mismatch: expected %s, got %s\n' "$BLESH_SOURCE_SHA256" "$actual" >&2
    exit 1
fi

if [[ ! -f $script ]]; then
    if ! command -v gawk >/dev/null 2>&1; then
        printf 'building the pinned ble.sh test artifact requires GNU awk (gawk)\n' >&2
        exit 1
    fi
    mkdir -p "$cache_root/source"
    tar -xzf "$archive" -C "$cache_root/source"
    make -C "$source_root" \
        FULLVER=0.4.0-nightly \
        BLE_GIT_COMMIT_ID=d69e4d5 \
        BLE_GIT_BRANCH=master
fi

reported=$({ source "$script" --version; } 2>&1)
if [[ $reported != *"version $BLESH_VERSION"* ]]; then
    printf 'built ble.sh version mismatch: expected %s, got %s\n' "$BLESH_VERSION" "$reported" >&2
    exit 1
fi

if [[ -n ${GITHUB_ENV-} ]]; then
    printf 'INTENT_SH_TEST_BLESH=%s\n' "$script" >> "$GITHUB_ENV"
fi
printf '%s\n' "$script"
