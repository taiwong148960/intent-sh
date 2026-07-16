#!/usr/bin/env bash

set -euo pipefail

one_line() {
    tr '\r\n' '  ' | sed 's/[^A-Za-z0-9._+,:=/() -]/?/g' | cut -c1-200
}

printf 'go=%s\n' "$(go version | one_line)"
printf 'os=%s\n' "$(uname -s | one_line)"
printf 'kernel=%s\n' "$(uname -r | one_line)"
printf 'architecture=%s\n' "$(uname -m | one_line)"
printf 'locale_charmap=%s\n' "$(locale charmap 2>/dev/null | one_line)"
printf 'term_fixture=xterm-256color,dumb\n'
for entry in \
    "bash:${INTENT_SH_TEST_BASH:-bash}" \
    "bash32:${INTENT_SH_TEST_BASH32:-}" \
    "zsh:zsh" \
    "tmux:${INTENT_SH_TEST_TMUX:-tmux}"
do
    name=${entry%%:*}
    command_name=${entry#*:}
    if [[ -n $command_name ]] && command -v "$command_name" >/dev/null 2>&1; then
        case $name in
            bash|bash32) value=$($command_name --version 2>/dev/null | sed -n '1p' | one_line) ;;
            zsh) value=$($command_name --version 2>/dev/null | one_line) ;;
            tmux) value=$($command_name -V 2>/dev/null | one_line) ;;
        esac
        printf '%s=%s\n' "$name" "$value"
    else
        printf '%s=unavailable\n' "$name"
    fi
done
if [[ -n ${INTENT_SH_TEST_BLESH_CACHE-} && -f ${INTENT_SH_TEST_BLESH_CACHE}/fixture/manifest ]]; then
    sed -n -E '/^(rootCommit|contribCommit|version|scriptSHA256)=/{s/[^A-Za-z0-9=.+-]/?/g;p;}' "${INTENT_SH_TEST_BLESH_CACHE}/fixture/manifest" | head -4
fi

if [[ -n ${GITHUB_ENV-} && -f ${GITHUB_ENV} && ! -L ${GITHUB_ENV} ]]; then
    bash_path=${INTENT_SH_TEST_BASH:-bash}
    if command -v "$bash_path" >/dev/null 2>&1; then
        bash_version=$($bash_path --noprofile --norc -c 'printf %s "$BASH_VERSION"' | sed 's/[^A-Za-z0-9._+()-]//g' | cut -c1-80)
        [[ -n $bash_version ]] && printf 'INTENT_SH_CI_BASH_VERSION=%s\n' "$bash_version" >> "$GITHUB_ENV"
    fi
    if command -v zsh >/dev/null 2>&1; then
        zsh_version=$(zsh -fc 'printf %s "$ZSH_VERSION"' | sed 's/[^A-Za-z0-9._+-]//g' | cut -c1-80)
        [[ -n $zsh_version ]] && printf 'INTENT_SH_CI_ZSH_VERSION=%s\n' "$zsh_version" >> "$GITHUB_ENV"
    fi
    tmux_path=${INTENT_SH_TEST_TMUX:-tmux}
    if command -v "$tmux_path" >/dev/null 2>&1; then
        tmux_version=$($tmux_path -V | awk '{print $2}' | sed 's/[^A-Za-z0-9._+-]//g' | cut -c1-80)
        [[ -n $tmux_version ]] && printf 'INTENT_SH_CI_TMUX_VERSION=%s\n' "$tmux_version" >> "$GITHUB_ENV"
    fi
fi
