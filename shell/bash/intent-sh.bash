# intent-sh Bash adapter (protocol 2)
# Loaded through: eval "$(intent-sh init bash)"

if [[ $- != *i* ]]; then
    :
else

__intent_sh_load_adapter() {

__intent_sh_requested_rewrite_key=__INTENT_SH_REWRITE_CANONICAL__
__intent_sh_requested_undo_key=__INTENT_SH_UNDO_CANONICAL__
__intent_sh_requested_rewrite_binding='__INTENT_SH_REWRITE_BINDING__'
__intent_sh_requested_undo_binding='__INTENT_SH_UNDO_BINDING__'
[[ $__intent_sh_requested_rewrite_key == __INTENT_SH_""REWRITE_CANONICAL__ ]] && __intent_sh_requested_rewrite_key=alt+g
[[ $__intent_sh_requested_undo_key == __INTENT_SH_""UNDO_CANONICAL__ ]] && __intent_sh_requested_undo_key=alt+u
[[ $__intent_sh_requested_rewrite_binding == __INTENT_SH_""REWRITE_BINDING__ ]] && __intent_sh_requested_rewrite_binding='\x1b\x67'
[[ $__intent_sh_requested_undo_binding == __INTENT_SH_""UNDO_BINDING__ ]] && __intent_sh_requested_undo_binding='\x1b\x75'

if [[ -n ${__intent_sh_loaded-} ]]; then
    if [[ ${__intent_sh_protocol_version-} != 2 ]]; then
        printf 'intent-sh: loaded adapter protocol %s is incompatible with protocol 2\n' "${__intent_sh_protocol_version-unknown}" >&2
        return 1 2>/dev/null || exit 1
    fi
    if [[ ${__intent_sh_rewrite_key-} != "$__intent_sh_requested_rewrite_key" || ${__intent_sh_undo_key-} != "$__intent_sh_requested_undo_key" ]]; then
        printf 'intent-sh: different rewrite or undo bindings are already active; start a new shell before loading the new configuration\n' >&2
        return 1 2>/dev/null || exit 1
    fi
    if [[ ${INTENT_SH_ADAPTER_READY-} == 1 ]]; then
        return 0 2>/dev/null || exit 0
    fi
fi

__intent_sh_protocol_version=2
__intent_sh_rewrite_key=$__intent_sh_requested_rewrite_key
__intent_sh_undo_key=$__intent_sh_requested_undo_key
__intent_sh_editor_backend=
__intent_sh_editor_version=
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

__intent_sh_set_status() {
    INTENT_SH_ADAPTER_PROTOCOL=2
    INTENT_SH_ADAPTER_BACKEND=$1
    INTENT_SH_ADAPTER_EDITOR_VERSION=$2
    INTENT_SH_ADAPTER_READY=$3
    INTENT_SH_ADAPTER_FAILURE=$4
    INTENT_SH_ADAPTER_CONFLICTS=$5
    export INTENT_SH_ADAPTER_PROTOCOL INTENT_SH_ADAPTER_BACKEND
    export INTENT_SH_ADAPTER_EDITOR_VERSION INTENT_SH_ADAPTER_READY
    export INTENT_SH_ADAPTER_FAILURE INTENT_SH_ADAPTER_CONFLICTS
    INTENT_SH_ADAPTER_REWRITE_KEY=$__intent_sh_rewrite_key
    INTENT_SH_ADAPTER_UNDO_KEY=$__intent_sh_undo_key
    export INTENT_SH_ADAPTER_REWRITE_KEY INTENT_SH_ADAPTER_UNDO_KEY
}

__intent_sh_fail_initialization() {
    __intent_sh_set_status "$1" "$2" 0 "$3" "$4"
    printf 'intent-sh: %s\n' "$5" >&2
    return 1
}

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

__intent_sh_capture_interrupt() {
    __intent_sh_interrupted=1
}

# The native trap and the terminal-byte monitor can race. An atomic directory
# claim makes exactly one of them responsible for forwarding cancellation to
# the adapter process; the adapter then tears down its provider process group.
__intent_sh_forward_interrupt() {
    __intent_sh_interrupted=1
    [[ -n ${__intent_sh_cancel_path-} && -n ${__intent_sh_provider_pid-} ]] || return 0
    if command mkdir -- "${__intent_sh_cancel_path}.lock" 2>/dev/null; then
        printf 'cancelled:%s\n' "${1:-signal}" > "$__intent_sh_cancel_path"
        builtin kill -CONT "$__intent_sh_provider_pid" 2>/dev/null || :
        builtin kill -INT "$__intent_sh_provider_pid" 2>/dev/null || :
    fi
}

__intent_sh_restore_hup_trap() {
    if [[ -n ${__intent_sh_previous_hup_trap-} ]]; then
        builtin eval "builtin $__intent_sh_previous_hup_trap"
    else
        builtin trap - HUP
    fi
}

__intent_sh_handle_hangup() {
    __intent_sh_forward_interrupt hangup
    if [[ -n ${__intent_sh_tty_monitor_pid-} ]]; then
        builtin kill -TERM "$__intent_sh_tty_monitor_pid" 2>/dev/null || :
    fi
    if [[ -n ${__intent_sh_tty_state-} ]]; then
        command stty "$__intent_sh_tty_state" < /dev/tty 2>/dev/null || :
    fi
    if [[ -n ${tmp-} ]]; then
        command rm -f -- "$tmp"
    fi
    if [[ -n ${__intent_sh_cancel_path-} ]]; then
        command rm -f -- "$__intent_sh_cancel_path"
        command rmdir -- "${__intent_sh_cancel_path}.lock" 2>/dev/null || :
    fi
    __intent_sh_restore_hup_trap
    builtin kill -HUP "$$"
}

__intent_sh_runtime_failure_message() {
    __intent_sh_runtime_message="the native Readline backend is no longer compatible; re-evaluate intent-sh init bash"
}

__intent_sh_mark_runtime_failure() {
    __intent_sh_set_status "$__intent_sh_editor_backend" "$__intent_sh_editor_version" 0 "$1" "$2"
}

__intent_sh_check_backend() {
    if ((BASH_VERSINFO[0] < 4)); then
        __intent_sh_mark_runtime_failure unsupported_bash ""
        return 1
    fi
    if [[ $__intent_sh_editor_backend != readline || ${INTENT_SH_ADAPTER_READY-} != 1 ]]; then
        __intent_sh_mark_runtime_failure missing_backend ""
        return 1
    fi
    return 0
}

__intent_sh_protocol_cursor_from_editor() {
    __intent_sh_protocol_cursor=$2
}

__intent_sh_place_cursor_at_end() {
    local value=${READLINE_LINE-}
    local LC_ALL=C
    READLINE_POINT=${#value}
}

__intent_sh_reset_native_continuation() {
    bind -x '"\C-^":__intent_sh_noop'
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
    if ! __intent_sh_check_backend; then
        __intent_sh_runtime_failure_message
        __intent_sh_message "$__intent_sh_runtime_message"
        return 0
    fi
    __intent_sh_protocol_cursor_from_editor "$current" "$current_cursor"
    local current_protocol_cursor=$__intent_sh_protocol_cursor
    local request_original='' request_previous=''
    local request_generation=0 pending_original=$current
    local pending_original_cursor=$current_cursor

    # A prior ordinary native acceptance maps the private continuation to
    # accept-line. Reset it before every rewrite so it cannot bypass a new
    # danger guard.
    __intent_sh_reset_native_continuation
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
    local __intent_sh_previous_int_trap=
    local __intent_sh_previous_hup_trap=
    local __intent_sh_tty_state=
    local __intent_sh_cancel_path=
    local __intent_sh_tty_monitor_pid=
    __intent_sh_tty_state=$(command stty -g < /dev/tty 2>/dev/null) || __intent_sh_tty_state=
    if [[ -n $__intent_sh_tty_state ]]; then
        __intent_sh_cancel_path=$(mktemp "${TMPDIR:-/tmp}/intent-sh-cancel.XXXXXXXX") || {
            command rm -f -- "$tmp"
            __intent_sh_active_request_id=
            __intent_sh_message "could not create a temporary cancellation marker"
            return 0
        }
        command stty -isig < /dev/tty 2>/dev/null
    fi
    local command_status=0
    __intent_sh_previous_hup_trap=$(builtin trap -p HUP)
    builtin trap '__intent_sh_handle_hangup' HUP
    # Use a scoped Bash trap while native Readline owns the interactive line.
    __intent_sh_previous_int_trap=$(builtin trap -p INT)
    builtin trap '__intent_sh_forward_interrupt process-signal' INT
    { printf '%s\0' \
            "$__intent_sh_protocol_version" \
            rewrite \
            bash \
            "$BASH_VERSION" \
            "$__intent_sh_editor_backend" \
            "$__intent_sh_editor_version" \
            "$current" \
            "$current_protocol_cursor" \
            "$request_original" \
            "$request_previous" \
            "$request_generation" \
            "$request_id"
    } | command intent-sh adapter rewrite --protocol "$__intent_sh_protocol_version" > "$tmp" &
    __intent_sh_provider_pid=$!
    if [[ -n $__intent_sh_tty_state ]]; then
        # Keep terminal polling in a monitor process so timed reads cannot
        # interrupt the interactive shell; it only waits and reaps jobs.
        # Disable monitor mode for this spawn so the reader stays in the
        # terminal's foreground process group and is allowed to read /dev/tty.
        local __intent_sh_monitor_mode=0
        if [[ $- == *m* ]]; then
            __intent_sh_monitor_mode=1
            set +m
        fi
        (
            while builtin kill -0 "$__intent_sh_provider_pid" 2>/dev/null; do
                __intent_sh_key=
                if IFS= builtin read -r -s -n 1 -t 1 __intent_sh_key < /dev/tty &&
                    [[ $__intent_sh_key == $'\003' ]]
                then
                    __intent_sh_forward_interrupt terminal-byte
                    break
                fi
            done
        ) &
        __intent_sh_tty_monitor_pid=$!
        if ((__intent_sh_monitor_mode)); then
            set -m
        fi
    fi
    __intent_sh_message "generating... (Ctrl+C to cancel)"
    local __intent_sh_wait_status=0
    while true; do
        __intent_sh_wait_status=0
        wait "$__intent_sh_provider_pid" || __intent_sh_wait_status=$?
        command_status=$__intent_sh_wait_status
        if ! kill -0 "$__intent_sh_provider_pid" 2>/dev/null; then
            break
        fi
        # A trapped signal can interrupt wait before the child changes state.
        # Forwarding is idempotent through the cancellation lock, then wait
        # again until the adapter and its provider descendants are reaped.
        if ((__intent_sh_interrupted)); then
            __intent_sh_forward_interrupt process-signal
        fi
    done
    if [[ -n $__intent_sh_tty_monitor_pid ]]; then
        builtin kill -TERM "$__intent_sh_tty_monitor_pid" 2>/dev/null
        wait "$__intent_sh_tty_monitor_pid" 2>/dev/null || :
    fi
    if [[ -n $__intent_sh_tty_state ]]; then
        command stty "$__intent_sh_tty_state" < /dev/tty 2>/dev/null
    fi
    if [[ -n $__intent_sh_cancel_path ]]; then
        [[ -s $__intent_sh_cancel_path ]] && __intent_sh_interrupted=1
        command rm -f -- "$__intent_sh_cancel_path"
        command rmdir -- "${__intent_sh_cancel_path}.lock" 2>/dev/null || :
    fi
    # Ignore a second INT during the small reporting window, then put back the
    # caller's exact trap below.
    builtin trap '' INT
    if ((__intent_sh_interrupted)); then
        __intent_sh_message "cancelled"
    fi
    if [[ -n $__intent_sh_previous_int_trap ]]; then
        builtin eval "builtin $__intent_sh_previous_int_trap"
    else
        builtin trap - INT
    fi
    __intent_sh_restore_hup_trap
    if ((__intent_sh_interrupted)); then
        command rm -f -- "$tmp"
        __intent_sh_active_request_id=
        READLINE_LINE=$current
        READLINE_POINT=$current_cursor
        return 0
    fi

    if ! __intent_sh_read_response "$tmp"; then
        command rm -f -- "$tmp"
        __intent_sh_active_request_id=
        READLINE_LINE=$current
        READLINE_POINT=$current_cursor
        if ((!__intent_sh_interrupted)); then
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
            __intent_sh_place_cursor_at_end
            __intent_sh_original_buffer=$pending_original
            __intent_sh_original_cursor=$pending_original_cursor
            __intent_sh_generated_command=$READLINE_LINE
            __intent_sh_provider=$__intent_sh_response_provider
            __intent_sh_risk=$__intent_sh_response_risk
            __intent_sh_risk_reason=$__intent_sh_response_risk_reason
            __intent_sh_request_id=$request_id
            __intent_sh_generation_index=$request_generation
            __intent_sh_armed_fingerprint=
            __intent_sh_reset_native_continuation
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
            __intent_sh_message "cancelled"
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
    if ! __intent_sh_check_backend; then
        __intent_sh_runtime_failure_message
        __intent_sh_message "$__intent_sh_runtime_message"
        return 0
    fi
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
    if ! __intent_sh_check_backend; then
        __intent_sh_runtime_failure_message
        __intent_sh_message "$__intent_sh_runtime_message"
        bind -x '"\C-^":__intent_sh_noop'
        return
    fi
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

__intent_sh_initialize_readline() {
    __intent_sh_editor_backend=readline
    __intent_sh_editor_version=$BASH_VERSION
    bind -x "\"$__intent_sh_requested_rewrite_binding\":__intent_sh_rewrite"
    bind -x "\"$__intent_sh_requested_undo_binding\":__intent_sh_undo"
    bind -x '"\C-]":__intent_sh_accept_guard'
    bind -x '"\C-^":__intent_sh_noop'
    bind '"\C-m":"\C-]\C-^"'
    bind '"\C-j":"\C-]\C-^"'
    __intent_sh_set_status readline "$__intent_sh_editor_version" 1 "" ""
    return 0
}

__intent_sh_initialize() {
    __intent_sh_set_status none none 0 initializing ""

    if ((BASH_VERSINFO[0] < 4)); then
        __intent_sh_fail_initialization none none unsupported_bash "" \
            "Bash 4.0 or newer is required; use Zsh or install a supported Bash"
        return 1
    fi

    __intent_sh_initialize_readline
    return $?
}

if ! __intent_sh_initialize; then
    return 1
fi

__intent_sh_loaded=1
return 0
}

__intent_sh_load_adapter
__intent_sh_load_status=$?
unset -f __intent_sh_load_adapter
if ((__intent_sh_load_status == 0)); then
    unset __intent_sh_load_status
    true
else
    unset __intent_sh_load_status
    false
fi

fi
