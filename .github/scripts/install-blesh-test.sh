#!/usr/bin/env bash

set -euo pipefail

script_directory=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
default_spec=$script_directory/../ci/blesh-fixture.env
spec=${INTENT_SH_TEST_BLESH_SPEC:-$default_spec}

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

is_regular_file() {
    [[ -f $1 && ! -L $1 ]]
}

spec_value() {
    local key=$1 value count
    count=$(grep -c "^${key}=" "$spec" || true)
    [[ $count == 1 ]] || fail "ble.sh fixture specification is missing or duplicates ${key}"
    value=$(sed -n "s/^${key}=//p" "$spec")
    [[ $value != *$'\n'* && -n $value ]] || fail "ble.sh fixture specification has an invalid ${key}"
    printf '%s\n' "$value"
}

is_regular_file "$spec" || fail "ble.sh fixture specification must be a regular file"

fixture_schema=$(spec_value BLESH_FIXTURE_SCHEMA)
installer_revision=$(spec_value BLESH_INSTALLER_REVISION)
root_commit=$(spec_value BLESH_ROOT_COMMIT)
root_url=$(spec_value BLESH_ROOT_ARCHIVE_URL)
root_sha256=$(spec_value BLESH_ROOT_ARCHIVE_SHA256)
contrib_commit=$(spec_value BLESH_CONTRIB_COMMIT)
contrib_url=$(spec_value BLESH_CONTRIB_ARCHIVE_URL)
contrib_sha256=$(spec_value BLESH_CONTRIB_ARCHIVE_SHA256)
expected_version=$(spec_value BLESH_VERSION)
root_license=$(spec_value BLESH_ROOT_LICENSE)
contrib_license=$(spec_value BLESH_CONTRIB_LICENSE)

[[ $fixture_schema =~ ^[0-9]+$ ]] || fail "ble.sh fixture schema is invalid"
[[ $installer_revision =~ ^[0-9]+$ ]] || fail "ble.sh installer revision is invalid"
[[ $root_commit =~ ^[0-9a-f]{40}$ && $contrib_commit =~ ^[0-9a-f]{40}$ ]] || fail "ble.sh fixture commit is invalid"
[[ $root_sha256 =~ ^[0-9a-f]{64}$ && $contrib_sha256 =~ ^[0-9a-f]{64}$ ]] || fail "ble.sh fixture checksum is invalid"
[[ $root_url == https://github.com/* || -n ${INTENT_SH_TEST_BLESH_SPEC-} ]] || fail "ble.sh root archive URL is invalid"
[[ $contrib_url == https://github.com/* || -n ${INTENT_SH_TEST_BLESH_SPEC-} ]] || fail "ble.sh contrib archive URL is invalid"
[[ $expected_version =~ ^[A-Za-z0-9.+-]{1,80}$ ]] || fail "ble.sh expected version is invalid"
[[ $root_license =~ ^[A-Za-z0-9.+-]{1,40}$ && $contrib_license =~ ^[A-Za-z0-9.+-]{1,40}$ ]] || fail "ble.sh license metadata is invalid"

cache_base=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
default_cache_root=${cache_base%/}/intent-sh-blesh
cache_root=${INTENT_SH_TEST_BLESH_CACHE:-$default_cache_root}
while [[ $cache_root == */ && $cache_root != / ]]; do
    cache_root=${cache_root%/}
done
[[ -n $cache_root && $cache_root != / && $cache_root != *$'\n'* ]] || fail "refusing unsafe ble.sh cache root"
if [[ -L $cache_root ]]; then
    fail "ble.sh cache root must not be a symlink"
fi
mkdir -p "$cache_root"
[[ -d $cache_root && ! -L $cache_root ]] || fail "ble.sh cache root is not a directory"

fixture=$cache_root/fixture
fixture_script=$fixture/ble.sh
fixture_manifest=$fixture/manifest
downloads=$cache_root/downloads
root_override=${INTENT_SH_TEST_BLESH_ROOT_ARCHIVE:-${INTENT_SH_TEST_BLESH_ARCHIVE:-}}
contrib_override=${INTENT_SH_TEST_BLESH_CONTRIB_ARCHIVE:-}
root_archive=${root_override:-$downloads/ble.sh-$root_commit.tar.gz}
contrib_archive=${contrib_override:-$downloads/blesh-contrib-$contrib_commit.tar.gz}

sha256_file() {
    local tool=${INTENT_SH_TEST_HASH_TOOL:-auto} output
    case $tool in
        auto)
            if command -v sha256sum >/dev/null 2>&1; then
                tool=sha256sum
            else
                tool=shasum
            fi
            ;;
        sha256sum|shasum) ;;
        *) fail "unsupported ble.sh checksum tool selection" ;;
    esac
    if [[ $tool == sha256sum ]]; then
        command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required for the selected checksum mode"
        output=$(sha256sum "$1")
    else
        command -v shasum >/dev/null 2>&1 || fail "shasum is required for the selected checksum mode"
        output=$(shasum -a 256 "$1")
    fi
    output=${output%%[[:space:]]*}
    [[ $output =~ ^[0-9a-f]{64}$ ]] || fail "checksum tool returned an invalid digest"
    printf '%s\n' "$output"
}

runtime_inventory() {
    local root=$1 inventory=$2 entry relative digest unsafe
    unsafe=$(find "$root" -mindepth 1 ! -type d ! -type f -print -quit)
    [[ -z $unsafe ]] || return 1
    : > "$inventory"
    while IFS= read -r entry; do
        relative=${entry#"$root"/}
        ((${#relative} <= 500)) && [[ $relative =~ ^[A-Za-z0-9_./+@%=-]+$ ]] || return 1
        is_regular_file "$entry" || return 1
        digest=$(sha256_file "$entry") || return 1
        printf '%s  %s\n' "$digest" "$relative" >> "$inventory"
    done < <(find "$root" -mindepth 1 -type f ! -path "$root/manifest" -print | LC_ALL=C sort)
    [[ -s $inventory ]]
}

cache_manifest_value() {
    local key=$1 count
    count=$(grep -c "^${key}=" "$fixture_manifest" || true)
    [[ $count == 1 ]] || return 1
    sed -n "s/^${key}=//p" "$fixture_manifest"
}

version_matches() {
    local candidate=$1 reported
    is_regular_file "$candidate" || return 1
    [[ $(wc -c < "$candidate") -le $((16 * 1024 * 1024)) ]] || return 1
    reported=$(/bin/bash --noprofile --norc -c 'source "$1" --version' _ "$candidate" 2>&1) || return 1
    [[ $reported == *"version $expected_version"* ]]
}

cache_is_valid() {
    cache_reason=missing-manifest
    is_regular_file "$fixture_manifest" || return 1
    cache_reason=manifest-shape
    [[ $(awk 'END { print NR }' "$fixture_manifest") == 12 ]] || return 1
    cache_reason='fixture-schema'
    [[ $(cache_manifest_value fixtureSchema) == "$fixture_schema" ]] || return 1
    cache_reason='installer-revision'
    [[ $(cache_manifest_value installerRevision) == "$installer_revision" ]] || return 1
    cache_reason='root-commit'
    [[ $(cache_manifest_value rootCommit) == "$root_commit" ]] || return 1
    cache_reason='root-checksum'
    [[ $(cache_manifest_value rootArchiveSHA256) == "$root_sha256" ]] || return 1
    cache_reason='contrib-commit'
    [[ $(cache_manifest_value contribCommit) == "$contrib_commit" ]] || return 1
    cache_reason='contrib-checksum'
    [[ $(cache_manifest_value contribArchiveSHA256) == "$contrib_sha256" ]] || return 1
    cache_reason=version
    [[ $(cache_manifest_value version) == "$expected_version" ]] || return 1
    cache_reason='root-license'
    [[ $(cache_manifest_value rootLicense) == "$root_license" ]] || return 1
    cache_reason='contrib-license'
    [[ $(cache_manifest_value contribLicense) == "$contrib_license" ]] || return 1
    local recorded_digest actual_digest recorded_file_count recorded_runtime_digest
    local inventory actual_file_count actual_runtime_digest required
    cache_reason='script-digest-record'
    recorded_digest=$(cache_manifest_value scriptSHA256) || return 1
    [[ $recorded_digest =~ ^[0-9a-f]{64}$ ]] || return 1
    cache_reason='script-file'
    is_regular_file "$fixture_script" || return 1
    actual_digest=$(sha256_file "$fixture_script")
    cache_reason='script-digest'
    [[ $actual_digest == "$recorded_digest" ]] || return 1
    for required in lib/init-bind.sh lib/keymap.emacs.sh lib/keymap.vi.sh; do
        cache_reason="runtime-$required"
        is_regular_file "$fixture/$required" || return 1
    done
    cache_reason='runtime-file-count-record'
    recorded_file_count=$(cache_manifest_value runtimeFileCount) || return 1
    [[ $recorded_file_count =~ ^[1-9][0-9]{0,5}$ ]] || return 1
    cache_reason='runtime-digest-record'
    recorded_runtime_digest=$(cache_manifest_value runtimeSHA256) || return 1
    [[ $recorded_runtime_digest =~ ^[0-9a-f]{64}$ ]] || return 1
    inventory=$(mktemp "$cache_root/.runtime-inventory.XXXXXX") || return 1
    if ! runtime_inventory "$fixture" "$inventory"; then
        rm -f -- "$inventory"
        cache_reason='runtime-tree'
        return 1
    fi
    actual_file_count=$(awk 'END { print NR }' "$inventory")
    actual_runtime_digest=$(sha256_file "$inventory")
    rm -f -- "$inventory"
    cache_reason='runtime-file-count'
    [[ $actual_file_count == "$recorded_file_count" ]] || return 1
    cache_reason='runtime-digest'
    [[ $actual_runtime_digest == "$recorded_runtime_digest" ]] || return 1
    cache_reason='script-version'
    version_matches "$fixture_script" || return 1
    cache_reason=valid
    return 0
}

export_fixture() {
    if [[ -n ${GITHUB_ENV-} ]]; then
        printf 'INTENT_SH_TEST_BLESH=%s\n' "$fixture_script" >> "$GITHUB_ENV"
    fi
    printf '%s\n' "$fixture_script"
}

if cache_is_valid; then
    export_fixture
    exit 0
fi

if [[ -L $downloads || ( -e $downloads && ! -d $downloads ) ]]; then
    rm -rf -- "$downloads"
fi
mkdir -p "$downloads"
[[ -d $downloads && ! -L $downloads ]] || fail "ble.sh download cache is not a real directory"

ensure_archive() {
    local label=$1 archive=$2 url=$3 expected=$4 override=$5 actual temporary
    if is_regular_file "$archive"; then
        actual=$(sha256_file "$archive")
        if [[ $actual == "$expected" ]]; then
            return 0
        fi
    fi
    if [[ -n $override ]]; then
        fail "$label archive is missing, non-regular, or has the wrong checksum"
    fi
    if [[ -e $archive || -L $archive ]]; then
        rm -f -- "$archive"
    fi
    temporary=$(mktemp "$downloads/.${label}.download.XXXXXX")
    if ! curl --fail --location --retry 3 --silent --show-error --output "$temporary" "$url"; then
        rm -f -- "$temporary"
        fail "could not download the pinned $label archive"
    fi
    is_regular_file "$temporary" || { rm -f -- "$temporary"; fail "downloaded $label archive is not a regular file"; }
    actual=$(sha256_file "$temporary")
    if [[ $actual != "$expected" ]]; then
        rm -f -- "$temporary"
        fail "downloaded $label archive checksum mismatch"
    fi
    mv -- "$temporary" "$archive"
}

ensure_archive root "$root_archive" "$root_url" "$root_sha256" "$root_override"
ensure_archive contrib "$contrib_archive" "$contrib_url" "$contrib_sha256" "$contrib_override"

command -v gawk >/dev/null 2>&1 || fail "building the pinned ble.sh test artifact requires GNU awk (gawk)"
work=$(mktemp -d "$cache_root/.build.XXXXXX")
cleanup() {
    rm -rf -- "$work"
}
trap cleanup EXIT HUP INT TERM

source_root=$work/source
mkdir -p "$source_root"
tar -xzf "$root_archive" -C "$source_root" --strip-components=1
is_regular_file "$source_root/GNUmakefile" || fail "pinned ble.sh root archive is incomplete"
if [[ -e $source_root/contrib || -L $source_root/contrib ]]; then
    rm -rf -- "$source_root/contrib"
fi
mkdir -p "$source_root/contrib"
tar -xzf "$contrib_archive" -C "$source_root/contrib" --strip-components=1
is_regular_file "$source_root/contrib/contrib.mk" || fail "pinned blesh-contrib archive is incomplete"

build_log=$work/build.log
if ! make -C "$source_root" \
    FULLVER=0.4.0-nightly \
    BLE_GIT_COMMIT_ID=${root_commit:0:7} \
    BLE_GIT_BRANCH=master >"$build_log" 2>&1; then
    fail "building the complete pinned ble.sh fixture failed"
fi
built_script=$source_root/out/ble.sh
version_matches "$built_script" || fail "built ble.sh version or file type did not match the fixture specification"
built_digest=$(sha256_file "$built_script")

publish=$work/publish
mkdir -p "$publish"
cp -R -L "$source_root/out/." "$publish/"
for required in ble.sh lib/init-bind.sh lib/keymap.emacs.sh lib/keymap.vi.sh; do
    is_regular_file "$publish/$required" || fail "built ble.sh runtime tree is incomplete"
done
runtime_inventory_file=$work/runtime.inventory
runtime_inventory "$publish" "$runtime_inventory_file" || fail "built ble.sh runtime tree contains an unsafe entry"
runtime_file_count=$(awk 'END { print NR }' "$runtime_inventory_file")
runtime_digest=$(sha256_file "$runtime_inventory_file")
cat > "$publish/manifest" <<EOF
fixtureSchema=$fixture_schema
installerRevision=$installer_revision
rootCommit=$root_commit
rootArchiveSHA256=$root_sha256
contribCommit=$contrib_commit
contribArchiveSHA256=$contrib_sha256
version=$expected_version
rootLicense=$root_license
contribLicense=$contrib_license
scriptSHA256=$built_digest
runtimeFileCount=$runtime_file_count
runtimeSHA256=$runtime_digest
EOF
chmod 0600 "$publish/manifest"

candidate=$cache_root/.fixture-candidate.$$
mv -- "$publish" "$candidate"
previous=$cache_root/.fixture-previous.$$
had_previous=0
if [[ -e $fixture || -L $fixture ]]; then
    mv -- "$fixture" "$previous"
    had_previous=1
fi
if ! mv -- "$candidate" "$fixture"; then
    if [[ $had_previous == 1 ]]; then
        mv -- "$previous" "$fixture"
    fi
    fail "could not atomically publish the ble.sh fixture"
fi
if [[ $had_previous == 1 ]]; then
    rm -rf -- "$previous"
fi
cache_is_valid || fail "published ble.sh fixture failed cache verification ($cache_reason)"
export_fixture
