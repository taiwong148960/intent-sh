#!/usr/bin/env bash

set -euo pipefail

script_directory=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
installer=$script_directory/install-blesh-test.sh
test_base=${TMPDIR:-/tmp}
test_root=$(mktemp -d "${test_base%/}/intent-sh-blesh-installer-test.XXXXXX")
trap 'rm -rf -- "$test_root"' EXIT HUP INT TERM

fail() {
    printf 'installer test failed: %s\n' "$1" >&2
    exit 1
}

hash_file() {
    if [[ $1 == sha256sum ]]; then
        sha256sum "$2" | awk '{print $1}'
    else
        shasum -a 256 "$2" | awk '{print $1}'
    fi
}

read_count() {
    if [[ -f $1 ]]; then
        tr -d '[:space:]' < "$1"
    else
        printf '0'
    fi
}

assert_count() {
    local actual
    actual=$(read_count "$1")
    [[ $actual == "$2" ]] || fail "build count was $actual, expected $2"
}

make_archives() {
    local directory=$1 mode=$2 root_source contrib_source
    root_source=$directory/root-source/root-package
    contrib_source=$directory/contrib-source/contrib-package
    mkdir -p "$root_source" "$contrib_source"
    if [[ $mode != missing-root ]]; then
        printf 'fixture makefile\n' > "$root_source/GNUmakefile"
    fi
    if [[ $mode != missing-contrib ]]; then
        printf 'fixture contrib\n' > "$contrib_source/contrib.mk"
    fi
    tar -czf "$directory/root.tar.gz" -C "$directory/root-source" root-package
    tar -czf "$directory/contrib.tar.gz" -C "$directory/contrib-source" contrib-package
}

write_spec() {
    local path=$1 revision=$2 root_archive=$3 contrib_archive=$4 hash_tool=$5
    local root_digest contrib_digest
    root_digest=$(hash_file "$hash_tool" "$root_archive")
    contrib_digest=$(hash_file "$hash_tool" "$contrib_archive")
    cat > "$path" <<EOF
BLESH_FIXTURE_SCHEMA=1
BLESH_INSTALLER_REVISION=$revision
BLESH_ROOT_COMMIT=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
BLESH_ROOT_ARCHIVE_URL=https://example.invalid/root.tar.gz
BLESH_ROOT_ARCHIVE_SHA256=$root_digest
BLESH_CONTRIB_COMMIT=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
BLESH_CONTRIB_ARCHIVE_URL=https://example.invalid/contrib.tar.gz
BLESH_CONTRIB_ARCHIVE_SHA256=$contrib_digest
BLESH_VERSION=0.4.0-test+fixture
BLESH_ROOT_LICENSE=BSD-3-Clause
BLESH_CONTRIB_LICENSE=BSD-3-Clause
EOF
}

make_fake_tools() {
    local directory=$1
    mkdir -p "$directory"
    cat > "$directory/gawk" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' 'GNU Awk 5.0 fixture'
EOF
    cat > "$directory/curl" <<'EOF'
#!/usr/bin/env bash
printf 'network access was attempted\n' > "${FAKE_CURL_MARKER:?}"
exit 99
EOF
    cat > "$directory/make" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
source_root=
while [[ $# -gt 0 ]]; do
    if [[ $1 == -C ]]; then
        source_root=$2
        shift 2
    else
        shift
    fi
done
[[ -n $source_root && -f $source_root/contrib/contrib.mk ]] || exit 40
count=0
if [[ -f ${FAKE_BUILD_COUNT:?} ]]; then
    count=$(tr -d '[:space:]' < "$FAKE_BUILD_COUNT")
fi
printf '%s\n' "$((count + 1))" > "$FAKE_BUILD_COUNT"
mkdir -p "$source_root/out/lib" "$source_root/out/contrib/integration"
if [[ ${FAKE_BUILD_FAIL:-0} == 1 ]]; then
    printf 'partial fixture\n' > "$source_root/out/ble.sh"
    exit 41
fi
cat > "$source_root/out/ble.sh" <<EOF_SCRIPT
if [[ \${1-} == --version ]]; then
    printf '%s\\n' 'ble.sh, version ${FAKE_BLESH_VERSION:?}'
    return 0 2>/dev/null || exit 0
fi
EOF_SCRIPT
printf 'fixture init bind\n' > "$source_root/out/lib/init-bind.sh"
printf 'fixture emacs keymap\n' > "$source_root/out/lib/keymap.emacs.sh"
printf 'fixture vi keymap\n' > "$source_root/out/lib/keymap.vi.sh"
printf 'fixture contrib runtime\n' > "$source_root/out/contrib/integration/runtime.bash"
EOF
    chmod 0755 "$directory/gawk" "$directory/curl" "$directory/make"
}

run_installer() {
    local fake_bin=$1 cache=$2 spec=$3 root_archive=$4 contrib_archive=$5 hash_tool=$6 count_file=$7 version=$8 build_fail=${9:-0}
    env \
        PATH="$fake_bin:/usr/bin:/bin:/usr/sbin:/sbin" \
        FAKE_BUILD_COUNT="$count_file" \
        FAKE_BUILD_FAIL="$build_fail" \
        FAKE_BLESH_VERSION="$version" \
        FAKE_CURL_MARKER="$cache/network-attempted" \
        INTENT_SH_TEST_BLESH_CACHE="$cache" \
        INTENT_SH_TEST_BLESH_SPEC="$spec" \
        INTENT_SH_TEST_BLESH_ROOT_ARCHIVE="$root_archive" \
        INTENT_SH_TEST_BLESH_CONTRIB_ARCHIVE="$contrib_archive" \
        INTENT_SH_TEST_HASH_TOOL="$hash_tool" \
        bash "$installer"
}

run_installer_with_default_cache() {
    local fake_bin=$1 temporary_base=$2 spec=$3 root_archive=$4 contrib_archive=$5 hash_tool=$6 count_file=$7 version=$8
    env -u RUNNER_TEMP -u INTENT_SH_TEST_BLESH_CACHE \
        PATH="$fake_bin:/usr/bin:/bin:/usr/sbin:/sbin" \
        TMPDIR="$temporary_base/" \
        FAKE_BUILD_COUNT="$count_file" \
        FAKE_BUILD_FAIL=0 \
        FAKE_BLESH_VERSION="$version" \
        FAKE_CURL_MARKER="$temporary_base/network-attempted" \
        INTENT_SH_TEST_BLESH_SPEC="$spec" \
        INTENT_SH_TEST_BLESH_ROOT_ARCHIVE="$root_archive" \
        INTENT_SH_TEST_BLESH_CONTRIB_ARCHIVE="$contrib_archive" \
        INTENT_SH_TEST_HASH_TOOL="$hash_tool" \
        bash "$installer"
}

exercise_variant() {
    local hash_tool=$1 directory fake_bin archives cache spec count_file output target target_digest old_digest stale_spec download_target
    local default_cache_base default_count
    directory=$test_root/$hash_tool
    fake_bin=$directory/bin
    archives=$directory/archives
    cache=$directory/cache
    spec=$directory/spec.env
    count_file=$directory/build-count
    mkdir -p "$archives"
    make_fake_tools "$fake_bin"
    make_archives "$archives" complete
    write_spec "$spec" 3 "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool"

    default_cache_base=$directory/default-cache-base
    default_count=$directory/default-cache-build-count
    mkdir -p "$default_cache_base"
    output=$(run_installer_with_default_cache "$fake_bin" "$default_cache_base" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$default_count" 0.4.0-test+fixture)
    [[ $output == "$default_cache_base/intent-sh-blesh/fixture/ble.sh" ]] || fail "$hash_tool trailing-slash TMPDIR produced an unclean default cache path"
    assert_count "$default_count" 1

    output=$(run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture)
    [[ $output == "$cache/fixture/ble.sh" && -f $output && ! -L $output ]] || fail "$hash_tool empty-cache publication failed"
    [[ ! -e $cache/network-attempted ]] || fail "$hash_tool empty-cache path used the network"
    assert_count "$count_file" 1

    output=$(run_installer "$fake_bin" "$cache/" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture)
    [[ $output == "$cache/fixture/ble.sh" ]] || fail "$hash_tool trailing-slash cache path was not normalized"
    assert_count "$count_file" 1

    mv "$archives/root.tar.gz" "$archives/root.saved"
    mv "$archives/contrib.tar.gz" "$archives/contrib.saved"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 1
    mv "$archives/root.saved" "$archives/root.tar.gz"
    mv "$archives/contrib.saved" "$archives/contrib.tar.gz"

    printf 'corruption\n' >> "$cache/fixture/ble.sh"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 2

    rm "$cache/fixture/lib/init-bind.sh"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 3

    awk 'BEGIN{done=0} /^installerRevision=/{print "installerRevision=stale"; done=1; next} {print} END{if(!done) exit 1}' \
        "$cache/fixture/manifest" > "$cache/fixture/manifest.new"
    mv "$cache/fixture/manifest.new" "$cache/fixture/manifest"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 4

    target=$directory/symlink-target
    cp "$cache/fixture/ble.sh" "$target"
    target_digest=$(hash_file "$hash_tool" "$target")
    rm "$cache/fixture/ble.sh"
    ln -s "$target" "$cache/fixture/ble.sh"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 5
    [[ ! -L $cache/fixture/ble.sh && $(hash_file "$hash_tool" "$target") == "$target_digest" ]] || fail "$hash_tool symlink cache repair was unsafe"

    download_target=$directory/download-symlink-target
    mkdir "$download_target"
    printf 'unchanged\n' > "$download_target/marker"
    rm -rf -- "$cache/downloads"
    ln -s "$download_target" "$cache/downloads"
    printf 'corruption\n' >> "$cache/fixture/ble.sh"
    run_installer "$fake_bin" "$cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture >/dev/null
    assert_count "$count_file" 6
    [[ -d $cache/downloads && ! -L $cache/downloads && $(cat "$download_target/marker") == unchanged ]] || fail "$hash_tool download-cache symlink repair was unsafe"

    stale_spec=$directory/spec-next.env
    write_spec "$stale_spec" 4 "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool"
    old_digest=$(hash_file "$hash_tool" "$cache/fixture/ble.sh")
    if run_installer "$fake_bin" "$cache" "$stale_spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$count_file" 0.4.0-test+fixture 1 >/dev/null 2>&1; then
        fail "$hash_tool failed build unexpectedly succeeded"
    fi
    [[ $(hash_file "$hash_tool" "$cache/fixture/ble.sh") == "$old_digest" ]] || fail "$hash_tool failed build replaced the prior fixture"
    [[ $(sed -n 's/^installerRevision=//p' "$cache/fixture/manifest") == 3 ]] || fail "$hash_tool failed build replaced the prior manifest"

    local incomplete=$directory/incomplete incomplete_spec=$directory/incomplete.env
    mkdir -p "$incomplete"
    make_archives "$incomplete" missing-contrib
    write_spec "$incomplete_spec" 3 "$incomplete/root.tar.gz" "$incomplete/contrib.tar.gz" "$hash_tool"
    if run_installer "$fake_bin" "$directory/incomplete-cache" "$incomplete_spec" "$incomplete/root.tar.gz" "$incomplete/contrib.tar.gz" "$hash_tool" "$directory/incomplete-count" 0.4.0-test+fixture >/dev/null 2>&1; then
        fail "$hash_tool incomplete contrib archive unexpectedly succeeded"
    fi
    [[ ! -e $directory/incomplete-cache/fixture ]] || fail "$hash_tool incomplete source published a fixture"

    local missing_root=$directory/missing-root missing_root_spec=$directory/missing-root.env
    mkdir -p "$missing_root"
    make_archives "$missing_root" missing-root
    write_spec "$missing_root_spec" 3 "$missing_root/root.tar.gz" "$missing_root/contrib.tar.gz" "$hash_tool"
    if run_installer "$fake_bin" "$directory/missing-root-cache" "$missing_root_spec" "$missing_root/root.tar.gz" "$missing_root/contrib.tar.gz" "$hash_tool" "$directory/missing-root-count" 0.4.0-test+fixture >/dev/null 2>&1; then
        fail "$hash_tool partial root archive unexpectedly succeeded"
    fi
    [[ ! -e $directory/missing-root-cache/fixture ]] || fail "$hash_tool partial extraction published a fixture"

    local corrupt_archive=$directory/corrupt-root.tar.gz
    cp "$archives/root.tar.gz" "$corrupt_archive"
    printf 'wrong digest\n' >> "$corrupt_archive"
    if run_installer "$fake_bin" "$directory/corrupt-cache" "$spec" "$corrupt_archive" "$archives/contrib.tar.gz" "$hash_tool" "$directory/corrupt-count" 0.4.0-test+fixture >/dev/null 2>&1; then
        fail "$hash_tool wrong archive digest unexpectedly succeeded"
    fi
    [[ ! -e $directory/corrupt-cache/fixture ]] || fail "$hash_tool wrong archive digest published a fixture"

    if run_installer "$fake_bin" "$directory/version-cache" "$spec" "$archives/root.tar.gz" "$archives/contrib.tar.gz" "$hash_tool" "$directory/version-count" 9.9.9-wrong >/dev/null 2>&1; then
        fail "$hash_tool wrong built version unexpectedly succeeded"
    fi
    [[ ! -e $directory/version-cache/fixture ]] || fail "$hash_tool wrong built version published a fixture"
}

for hash_tool in sha256sum shasum; do
    command -v "$hash_tool" >/dev/null 2>&1 || fail "$hash_tool is required for installer portability tests"
    exercise_variant "$hash_tool"
done

printf '%s\n' 'ble.sh installer tests passed for sha256sum and shasum modes'
