# intent-sh Bash adapter (protocol 1)
# Loaded through: eval "$(intent-sh init bash)"

if [[ $- != *i* ]]; then
    return 0 2>/dev/null || exit 0
fi

if ((BASH_VERSINFO[0] < 4)); then
    printf 'intent-sh: Bash 4.0 or newer is required (this is Bash %s). Use Zsh or install a modern Bash.\n' "$BASH_VERSION" >&2
    return 1 2>/dev/null || exit 1
fi

if [[ -n ${__intent_sh_loaded-} ]]; then
    if [[ ${__intent_sh_protocol_version-} != 1 ]]; then
        printf 'intent-sh: loaded adapter protocol %s is incompatible with protocol 1\n' "${__intent_sh_protocol_version-unknown}" >&2
        return 1 2>/dev/null || exit 1
    fi
    return 0 2>/dev/null || exit 0
fi

__intent_sh_loaded=1
__intent_sh_protocol_version=1
__intent_sh_original_buffer=
__intent_sh_original_cursor=0
__intent_sh_generated_command=
__intent_sh_provider=
__intent_sh_risk=
__intent_sh_risk_reason=
__intent_sh_request_id=
__intent_sh_active_request_id=
__intent_sh_generation_index=0
__intent_sh_request_counter=0
__intent_sh_armed_fingerprint=

__intent_sh_clear_chain() {
    __intent_sh_original_buffer=
    __intent_sh_original_cursor=0
    __intent_sh_generated_command=
    __intent_sh_provider=
    __intent_sh_risk=
    __intent_sh_risk_reason=
    __intent_sh_request_id=
    __intent_sh_generation_index=0
    __intent_sh_armed_fingerprint=
}

__intent_sh_message() {
    printf '\nintent-sh: %s\n' "$1"
}

__intent_sh_read_response() {
    local path=$1 parse_ok=1 extra=
    __intent_sh_response_version=
    __intent_sh_response_status=
    __intent_sh_response_replacement=
    __intent_sh_response_message=
    __intent_sh_response_provider=
    __intent_sh_response_risk=
    __intent_sh_response_risk_reason=
    __intent_sh_response_request_id=
    {
        IFS= read -r -d '' __intent_sh_response_version || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_status || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_replacement || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_message || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_provider || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_risk || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_risk_reason || parse_ok=0
        IFS= read -r -d '' __intent_sh_response_request_id || parse_ok=0
        if IFS= read -r -d '' extra || [[ -n $extra ]]; then
            parse_ok=0
        fi
    } < "$path"
    ((parse_ok))
}

__intent_sh_rewrite() {
    local current=${READLINE_LINE-}
    local current_cursor=${READLINE_POINT-0}
    local request_original= request_previous=
    local request_generation=0 pending_original=$current
    local pending_original_cursor=$current_cursor

    # A prior ordinary acceptance maps the private continuation to accept-line.
    # Reset it before every rewrite so it cannot bypass a new danger guard.
    bind -x '"\C-^":__intent_sh_noop'
    __intent_sh_armed_fingerprint=
    if [[ -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" && -n $__intent_sh_original_buffer ]]; then
        request_original=$__intent_sh_original_buffer
        request_previous=$__intent_sh_generated_command
        request_generation=$((__intent_sh_generation_index + 1))
        pending_original=$__intent_sh_original_buffer
        pending_original_cursor=$__intent_sh_original_cursor
    else
        __intent_sh_clear_chain
    fi

    if [[ -z ${current//[[:space:]]/} ]]; then
        __intent_sh_message "enter a command or intent before requesting a rewrite"
        return 0
    fi

    if ! command -v intent-sh >/dev/null 2>&1; then
        __intent_sh_message "binary not found on PATH"
        return 0
    fi

    __intent_sh_request_counter=$((__intent_sh_request_counter + 1))
    local request_id="bash-$$-${__intent_sh_request_counter}-${RANDOM}"
    __intent_sh_active_request_id=$request_id
    local tmp
    tmp=$(mktemp "${TMPDIR:-/tmp}/intent-sh-bash.XXXXXXXX") || {
        __intent_sh_active_request_id=
        __intent_sh_message "could not create a temporary response file"
        return 0
    }

    local __intent_sh_interrupted=0
    local __intent_sh_provider_pid=
    local __intent_sh_cancel_message_shown=0
    local __intent_sh_previous_int_trap
    __intent_sh_previous_int_trap=$(trap -p INT)
    local __intent_sh_tty_state=
    __intent_sh_tty_state=$(command stty -g < /dev/tty 2>/dev/null) || __intent_sh_tty_state=
    if [[ -n $__intent_sh_tty_state ]]; then
        command stty -isig < /dev/tty 2>/dev/null
    fi
    local command_status=0
    { printf '%s\0' \
            "$__intent_sh_protocol_version" \
            rewrite \
            bash \
            "$BASH_VERSION" \
            "$current" \
            "$current_cursor" \
            "$request_original" \
            "$request_previous" \
            "$request_generation" \
            "$request_id"
    } | command intent-sh adapter rewrite --protocol "$__intent_sh_protocol_version" > "$tmp" &
    __intent_sh_provider_pid=$!
    trap '__intent_sh_interrupted=1; if [[ -n $__intent_sh_provider_pid ]]; then kill -INT "$__intent_sh_provider_pid" 2>/dev/null; fi; if ((!__intent_sh_cancel_message_shown)); then __intent_sh_cancel_message_shown=1; __intent_sh_message "cancelled"; fi' INT
    __intent_sh_message "generating... (Ctrl+C to cancel)"
    if [[ -n $__intent_sh_tty_state ]]; then
        while kill -0 "$__intent_sh_provider_pid" 2>/dev/null; do
            local __intent_sh_key=
            if IFS= read -r -s -n 1 -t 0.05 __intent_sh_key < /dev/tty && [[ $__intent_sh_key == $'\003' ]]; then
                __intent_sh_interrupted=1
                kill -INT "$__intent_sh_provider_pid" 2>/dev/null
            fi
        done
        wait "$__intent_sh_provider_pid" || command_status=$?
        command stty "$__intent_sh_tty_state" < /dev/tty 2>/dev/null
    else
        wait "$__intent_sh_provider_pid" || command_status=$?
    fi
    if [[ -n $__intent_sh_previous_int_trap ]]; then
        builtin eval "$__intent_sh_previous_int_trap"
    else
        trap - INT
    fi

    if ! __intent_sh_read_response "$tmp"; then
        command rm -f -- "$tmp"
        __intent_sh_active_request_id=
        READLINE_LINE=$current
        READLINE_POINT=$current_cursor
        if ((__intent_sh_interrupted)); then
            if ((!__intent_sh_cancel_message_shown)); then
                __intent_sh_message "cancelled"
            fi
        else
            __intent_sh_message "received a malformed adapter response"
        fi
        return 0
    fi
    command rm -f -- "$tmp"

    if [[ $__intent_sh_response_version != "$__intent_sh_protocol_version" ]]; then
        __intent_sh_active_request_id=
        READLINE_LINE=$current
        READLINE_POINT=$current_cursor
        __intent_sh_message "adapter protocol mismatch"
        return 0
    fi
    if [[ $__intent_sh_response_request_id != "$request_id" || $__intent_sh_active_request_id != "$request_id" ]]; then
        __intent_sh_active_request_id=
        READLINE_LINE=$current
        READLINE_POINT=$current_cursor
        __intent_sh_message "ignored a stale adapter response"
        return 0
    fi
    __intent_sh_active_request_id=

    case $__intent_sh_response_status in
        ok)
            if [[ -z $__intent_sh_response_replacement ]]; then
                READLINE_LINE=$current
                READLINE_POINT=$current_cursor
                __intent_sh_message "adapter returned an empty replacement"
                return 0
            fi
            READLINE_LINE=$__intent_sh_response_replacement
            READLINE_POINT=${#READLINE_LINE}
            __intent_sh_original_buffer=$pending_original
            __intent_sh_original_cursor=$pending_original_cursor
            __intent_sh_generated_command=$READLINE_LINE
            __intent_sh_provider=$__intent_sh_response_provider
            __intent_sh_risk=$__intent_sh_response_risk
            __intent_sh_risk_reason=$__intent_sh_response_risk_reason
            __intent_sh_request_id=$request_id
            __intent_sh_generation_index=$request_generation
            __intent_sh_armed_fingerprint=
            bind -x '"\C-^":__intent_sh_noop'
            if [[ $__intent_sh_risk == dangerous ]]; then
                __intent_sh_message "DANGEROUS: ${__intent_sh_risk_reason:-review carefully}; first Enter warns, second unchanged Enter executes"
            elif [[ $__intent_sh_risk == review ]]; then
                __intent_sh_message "REVIEW: ${__intent_sh_risk_reason:-review before executing}"
            elif [[ -n $__intent_sh_response_message ]]; then
                __intent_sh_message "$__intent_sh_response_message"
            else
                __intent_sh_message "command inserted; review it before pressing Enter"
            fi
            ;;
        clarify)
            READLINE_LINE=$current
            READLINE_POINT=$current_cursor
            __intent_sh_message "${__intent_sh_response_message:-more detail is required}"
            ;;
        cancelled)
            READLINE_LINE=$current
            READLINE_POINT=$current_cursor
            if ((!__intent_sh_cancel_message_shown)); then
                __intent_sh_message "cancelled"
            fi
            ;;
        error)
            READLINE_LINE=$current
            READLINE_POINT=$current_cursor
            __intent_sh_message "${__intent_sh_response_message:-rewrite failed}"
            ;;
        *)
            READLINE_LINE=$current
            READLINE_POINT=$current_cursor
            __intent_sh_message "adapter returned an unknown status (exit $command_status)"
            ;;
    esac
    return 0
}

__intent_sh_undo() {
    local current=${READLINE_LINE-}
    if [[ -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" && -n $__intent_sh_original_buffer ]]; then
        local restored=$__intent_sh_original_buffer
        local restored_cursor=$__intent_sh_original_cursor
        __intent_sh_clear_chain
        READLINE_LINE=$restored
        READLINE_POINT=$restored_cursor
        __intent_sh_message "restored the original buffer"
        return 0
    fi
    if [[ -n $__intent_sh_generated_command && $current != "$__intent_sh_generated_command" ]]; then
        __intent_sh_clear_chain
        __intent_sh_message "buffer was edited; undo did not overwrite it"
        return 0
    fi
    __intent_sh_message "nothing to restore"
}

__intent_sh_noop() {
    :
}

__intent_sh_accept_guard() {
    local current=${READLINE_LINE-}
    if [[ -n $__intent_sh_generated_command && $current != "$__intent_sh_generated_command" ]]; then
        __intent_sh_clear_chain
        bind '"\C-^": accept-line'
        return
    fi
    if [[ $__intent_sh_risk == dangerous && -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" ]]; then
        if [[ $__intent_sh_armed_fingerprint == "$current" ]]; then
            __intent_sh_clear_chain
            bind '"\C-^": accept-line'
            return
        fi
        __intent_sh_armed_fingerprint=$current
        bind -x '"\C-^":__intent_sh_noop'
        __intent_sh_message "DANGEROUS: ${__intent_sh_risk_reason:-dangerous command}. Press Enter again to execute."
        return
    fi
    __intent_sh_clear_chain
    bind '"\C-^": accept-line'
}

# Enter expands to a guard callback and then a continuation. The callback maps
# the continuation to native accept-line or a no-op for this keypress.
bind -x '"\eg":__intent_sh_rewrite'
bind -x '"\eu":__intent_sh_undo'
bind -x '"\C-]":__intent_sh_accept_guard'
bind -x '"\C-^":__intent_sh_noop'
bind '"\C-m":"\C-]\C-^"'
bind '"\C-j":"\C-]\C-^"'
