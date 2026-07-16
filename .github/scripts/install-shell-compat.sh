#!/usr/bin/env bash

set -euo pipefail
umask 077

fixture_file=.github/ci/shell-compat-fixtures.env
if [[ ! -f $fixture_file || -L $fixture_file ]]; then
  printf 'shell compatibility fixture metadata is missing or unsafe\n' >&2
  exit 1
fi
# shellcheck source=.github/ci/shell-compat-fixtures.env
source "$fixture_file"

fixture=${INTENT_SH_SHELL_FIXTURE-}
case $fixture in
  bash-4.0)
    name=bash version=$BASH_40_VERSION url=$BASH_40_URL expected_sha=$BASH_40_SHA256 archive_name=bash-4.0.tar.gz source_name=bash-4.0
    ;;
  bash-5.3)
    name=bash version=$BASH_53_VERSION url=$BASH_53_URL expected_sha=$BASH_53_SHA256 archive_name=bash-5.3.tar.gz source_name=bash-5.3
    ;;
  zsh-5.8.1)
    name=zsh version=$ZSH_581_VERSION url=$ZSH_581_URL expected_sha=$ZSH_581_SHA256 archive_name=zsh-5.8.1.tar.xz source_name=zsh-5.8.1
    ;;
  zsh-5.9.1)
    name=zsh version=$ZSH_591_VERSION url=$ZSH_591_URL expected_sha=$ZSH_591_SHA256 archive_name=zsh-5.9.1.tar.xz source_name=zsh-5.9.1
    ;;
  *)
    printf 'INTENT_SH_SHELL_FIXTURE must select a checked-in fixture\n' >&2
    exit 1
    ;;
esac

cache_root=${INTENT_SH_SHELL_COMPAT_CACHE:-${RUNNER_TEMP:-/tmp}/intent-sh-shell-$fixture}
if (( ${#cache_root} > 500 )) || [[ ! $cache_root =~ ^/[A-Za-z0-9._/+@%-]+$ || $cache_root == / || $cache_root == *'/../'* || $cache_root == *'/./'* ]]; then
  printf 'shell compatibility cache path is unsafe\n' >&2
  exit 1
fi
if [[ -e $cache_root && ( -L $cache_root || ! -d $cache_root ) ]]; then
  printf 'shell compatibility cache must be a real directory\n' >&2
  exit 1
fi
fixture_root=$cache_root/fixture
manifest=$fixture_root/manifest
binary=$fixture_root/bin/$name

sha256_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

script_sha=$(sha256_file "$0")
valid_cache() {
  [[ -f $manifest && ! -L $manifest && -f $binary && ! -L $binary && -x $binary ]] || return 1
  grep -F -x -q "schema=1" "$manifest" || return 1
  grep -F -x -q "fixture=$fixture" "$manifest" || return 1
  grep -F -x -q "version=$version" "$manifest" || return 1
  grep -F -x -q "sourceSHA256=$expected_sha" "$manifest" || return 1
  grep -F -x -q "installerRevision=$SHELL_COMPAT_INSTALLER_REVISION" "$manifest" || return 1
  grep -F -x -q "installerSHA256=$script_sha" "$manifest" || return 1
  expected_binary_sha=$(sed -n 's/^binarySHA256=//p' "$manifest")
  [[ $expected_binary_sha =~ ^[a-f0-9]{64}$ && $(sha256_file "$binary") == "$expected_binary_sha" ]] || return 1
  actual_version=$($binary --noprofile --norc -c 'printf %s "$BASH_VERSION"' 2>/dev/null || true)
  if [[ $name == zsh ]]; then
    actual_version=$($binary -fc 'printf %s "$ZSH_VERSION"' 2>/dev/null || true)
  fi
  [[ $actual_version == "$version"* ]]
}

if ! valid_cache; then
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/intent-sh-shell-compat.XXXXXXXX")
  trap 'rm -rf -- "$temporary"' EXIT
  archive=$temporary/$archive_name
  curl --ipv4 --fail --location --retry 3 --connect-timeout 20 --max-time 180 --silent --show-error --proto '=https' --tlsv1.2 --output "$archive" "$url"
  actual_sha=$(sha256_file "$archive")
  [[ $actual_sha == "$expected_sha" ]] || {
    printf 'shell compatibility source checksum mismatch\n' >&2
    exit 1
  }
  mkdir "$temporary/source" "$temporary/staging"
  tar -xf "$archive" -C "$temporary/source"
  source_root=$temporary/source/$source_name
  [[ -d $source_root && ! -L $source_root ]] || {
    printf 'shell compatibility archive root is incomplete\n' >&2
    exit 1
  }
  build_log=$temporary/build.log
  if ! (
    cd "$source_root"
    if [[ $fixture == bash-4.0 ]]; then
      export CPPFLAGS="${CPPFLAGS:+$CPPFLAGS }-include sys/ioctl.h"
      export CFLAGS="${CFLAGS:+$CFLAGS }-Wno-error=implicit-function-declaration -Wno-error=implicit-int"
    fi
    if [[ $fixture == zsh-5.8.1 ]]; then
      # Its generated configure checks predate Clang's implicit-declaration
      # error and otherwise mis-detect the macOS process APIs they exercise.
      export CFLAGS="${CFLAGS:+$CFLAGS }-Wno-implicit-function-declaration -Wno-implicit-int"
    fi
    if [[ $name == bash ]]; then
      ./configure --prefix="$fixture_root" --without-bash-malloc
      make -j4
      make install DESTDIR="$temporary/staging"
    else
      ./configure --prefix="$fixture_root" --disable-gdbm --enable-multibyte --with-tcsetpgrp DL_EXT=bundle
      make -j4
      make install.bin install.modules DESTDIR="$temporary/staging"
    fi
  ) >"$build_log" 2>&1; then
    grep -n -C 2 'error:' "$build_log" | tail -n 80 >&2 || tail -n 80 "$build_log" >&2
    printf 'shell compatibility source build failed\n' >&2
    exit 1
  fi
  staged_prefix=$temporary/staging$fixture_root
  built=$staged_prefix/bin/$name
  [[ -f $built && ! -L $built && -x $built ]] || {
    printf 'shell compatibility build omitted its executable\n' >&2
    exit 1
  }
  publication=$temporary/publication
  mkdir "$publication"
  cp -R "$staged_prefix/." "$publication/"
  chmod 700 "$publication/bin/$name"
  binary_sha=$(sha256_file "$publication/bin/$name")
  {
    printf 'schema=1\n'
    printf 'fixture=%s\n' "$fixture"
    printf 'version=%s\n' "$version"
    printf 'sourceSHA256=%s\n' "$expected_sha"
    printf 'binarySHA256=%s\n' "$binary_sha"
    printf 'installerRevision=%s\n' "$SHELL_COMPAT_INSTALLER_REVISION"
    printf 'installerSHA256=%s\n' "$script_sha"
    printf 'license=GPL-3.0-or-later\n'
  } > "$publication/manifest"
  mkdir -p "$cache_root"
  previous=$cache_root/fixture.previous
  rm -rf -- "$previous"
  had_previous=0
  if [[ -e $fixture_root || -L $fixture_root ]]; then
    mv -- "$fixture_root" "$previous"
    had_previous=1
  fi
  if ! mv -- "$publication" "$fixture_root"; then
    if [[ $had_previous == 1 ]]; then
      mv -- "$previous" "$fixture_root"
    fi
    printf 'could not atomically publish shell compatibility fixture\n' >&2
    exit 1
  fi
  rm -rf -- "$previous"
  valid_cache || {
    printf 'published shell compatibility cache did not validate\n' >&2
    exit 1
  }
fi

if [[ -n ${GITHUB_ENV-} ]]; then
  printf 'INTENT_SH_TEST_COMPAT_NAME=%s\n' "$name" >> "$GITHUB_ENV"
  printf 'INTENT_SH_TEST_COMPAT_PATH=%s\n' "$binary" >> "$GITHUB_ENV"
  printf 'INTENT_SH_TEST_COMPAT_FIXTURE=%s\n' "$fixture" >> "$GITHUB_ENV"
else
  printf 'INTENT_SH_TEST_COMPAT_NAME=%s\n' "$name"
  printf 'INTENT_SH_TEST_COMPAT_PATH=%s\n' "$binary"
fi
