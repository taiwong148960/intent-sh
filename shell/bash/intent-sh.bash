# intent-sh Bash adapter (protocol 2)
# Loaded through: eval "$(intent-sh init bash)"

if [[ $- != *i* ]]; then
    :
else

__intent_sh_load_adapter() {

__intent_sh_expected_blesh_version=0.4.0-nightly+d69e4d5
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
__intent_sh_blesh_keymap=
: "${__intent_sh_blesh_advice_installed:=}"
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
    case ${INTENT_SH_ADAPTER_FAILURE-} in
        detached)
            __intent_sh_runtime_message="ble.sh detached; reattach it, then re-evaluate intent-sh init bash"
            ;;
        wrong_load_order)
            __intent_sh_runtime_message="the Bash editor changed; load ble.sh first, then re-evaluate intent-sh init bash"
            ;;
        incompatible_version)
            __intent_sh_runtime_message="ble.sh changed to an untested version; load $__intent_sh_expected_blesh_version and reinitialize"
            ;;
        keymap_changed)
            __intent_sh_runtime_message="the ble.sh keymap changed; re-evaluate intent-sh init bash in Emacs or Vi insert mode"
            ;;
        *)
            __intent_sh_runtime_message="the selected editor backend is no longer compatible; re-evaluate intent-sh init bash"
            ;;
    esac
}

__intent_sh_require_blesh_api() {
    local name
    for name in \
        ble-bind \
        blehook \
        ble/function#advice \
        ble/function#advice/do \
        ble/widget/default/accept-line \
        ble/widget/print \
        ble/widget/.EDIT_COMMAND
    do
        if ! declare -F "$name" >/dev/null 2>&1; then
            return 1
        fi
    done
    return 0
}

__intent_sh_mark_runtime_failure() {
    __intent_sh_set_status "$__intent_sh_editor_backend" "$__intent_sh_editor_version" 0 "$1" "$2"
}

__intent_sh_check_backend() {
    case $__intent_sh_editor_backend in
        blesh)
            if [[ ${INTENT_SH_ADAPTER_READY-} != 1 ]]; then
                return 1
            fi
            if [[ ${BLE_ATTACHED-} != 1 ]]; then
                __intent_sh_mark_runtime_failure detached ""
                return 1
            fi
            if [[ ${BLE_VERSION-} != "$__intent_sh_expected_blesh_version" ]]; then
                __intent_sh_mark_runtime_failure incompatible_version ""
                return 1
            fi
            if ! __intent_sh_require_blesh_api; then
                __intent_sh_mark_runtime_failure missing_api ""
                return 1
            fi
            if [[ ${_ble_decode_keymap-} != "$__intent_sh_blesh_keymap" ]]; then
                __intent_sh_mark_runtime_failure keymap_changed ""
                return 1
            fi
            if [[ $__intent_sh_blesh_advice_installed != 1 ]] ||
                ! declare -F ble/function#advice/around:ble/widget/default/accept-line >/dev/null 2>&1
            then
                __intent_sh_mark_runtime_failure missing_api "accept-line"
                return 1
            fi
            ;;
        readline)
            if ((BASH_VERSINFO[0] < 4)); then
                __intent_sh_mark_runtime_failure unsupported_bash ""
                return 1
            fi
            if [[ ${BLE_ATTACHED-} == 1 ]]; then
                __intent_sh_mark_runtime_failure wrong_load_order ""
                return 1
            fi
            ;;
        *)
            __intent_sh_mark_runtime_failure missing_backend ""
            return 1
            ;;
    esac
    return 0
}

__intent_sh_protocol_cursor_from_editor() {
    local line=$1 editor_cursor=$2
    if [[ $__intent_sh_editor_backend == blesh ]]; then
        local prefix=${line:0:editor_cursor}
        local LC_ALL=C
        __intent_sh_protocol_cursor=${#prefix}
    else
        __intent_sh_protocol_cursor=$editor_cursor
    fi
}

__intent_sh_place_cursor_at_end() {
    local value=${READLINE_LINE-}
    if [[ $__intent_sh_editor_backend == blesh ]]; then
        READLINE_POINT=${#value}
    else
        local LC_ALL=C
        READLINE_POINT=${#value}
    fi
}

__intent_sh_reset_native_continuation() {
    if [[ $__intent_sh_editor_backend == readline ]]; then
        bind -x '"\C-^":__intent_sh_noop'
    fi
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
    local __intent_sh_signal_mode=trap
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
    # Keep asynchronous signal handling deliberately minimal. ble.sh owns the
    # real INT trap while its editor is attached, so use its supported hook
    # registry instead of replacing that trap from inside an edit widget.
    # Native Readline has no such dispatcher and uses a scoped Bash trap.
    if [[ $__intent_sh_editor_backend == blesh ]]; then
        __intent_sh_signal_mode=blesh
        blehook 'INT-=__intent_sh_forward_interrupt' >/dev/null 2>&1 || :
        blehook 'INT!=__intent_sh_forward_interrupt'
    else
        __intent_sh_previous_int_trap=$(builtin trap -p INT)
        builtin trap '__intent_sh_forward_interrupt process-signal' INT
    fi
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
        # Bash 3.2 can segfault when SIGINT interrupts its timed `read` builtin
        # from inside a ble.sh edit widget. Run terminal polling in a monitor
        # process instead; the interactive shell only waits and reaps jobs.
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
    if [[ $__intent_sh_signal_mode == blesh ]]; then
        blehook 'INT-=__intent_sh_forward_interrupt' >/dev/null 2>&1 || :
    else
        # Ignore a second INT during the small reporting window, then put back
        # the caller's exact trap below.
        builtin trap '' INT
    fi
    if ((__intent_sh_interrupted)); then
        __intent_sh_message "cancelled"
    fi
    if [[ $__intent_sh_signal_mode != blesh ]]; then
        if [[ -n $__intent_sh_previous_int_trap ]]; then
            builtin eval "builtin $__intent_sh_previous_int_trap"
        else
            builtin trap - INT
        fi
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

__intent_sh_blesh_accept_guard() {
    local current=${_ble_edit_str-}
    __intent_sh_blesh_accept_action=block
    if ! __intent_sh_check_backend; then
        __intent_sh_clear_chain
        __intent_sh_runtime_failure_message
        ble/widget/print "intent-sh: $__intent_sh_runtime_message"
        return 0
    fi
    if [[ -n $__intent_sh_generated_command && $current != "$__intent_sh_generated_command" ]]; then
        __intent_sh_clear_chain
        __intent_sh_blesh_accept_action=delegate
        return 0
    fi
    if [[ $__intent_sh_risk == dangerous && -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" ]]; then
        if [[ $__intent_sh_armed_fingerprint == "$current" ]]; then
            __intent_sh_clear_chain
            __intent_sh_blesh_accept_action=delegate
            return 0
        fi
        __intent_sh_armed_fingerprint=$current
        ble/widget/print "intent-sh: DANGEROUS: ${__intent_sh_risk_reason:-dangerous command}. Press Enter again to execute."
        return 0
    fi
    __intent_sh_clear_chain
    __intent_sh_blesh_accept_action=delegate
}

__intent_sh_blesh_detach() {
    __intent_sh_clear_chain
    __intent_sh_set_status blesh "$__intent_sh_expected_blesh_version" 0 detached ""
    if declare -F ble/function#advice >/dev/null 2>&1; then
        ble/function#advice remove ble/widget/default/accept-line >/dev/null 2>&1 || :
    fi
    __intent_sh_blesh_advice_installed=
}

__intent_sh_blesh_binding_state() {
    local key=$1 default_widget=$2 callback=$3 line
    __intent_sh_binding_state=unbound
    while IFS= read -r line; do
        if [[ $line == "ble-bind -m '$__intent_sh_blesh_keymap' -f $key '$default_widget'" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -f '$key' '$default_widget'" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -f $key $default_widget" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -f '$key' $default_widget" ]]; then
            __intent_sh_binding_state=default
        elif [[ $line == "ble-bind -m '$__intent_sh_blesh_keymap' -x $key '$callback'" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -x '$key' '$callback'" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -x $key $callback" ||
            $line == "ble-bind -m '$__intent_sh_blesh_keymap' -x '$key' $callback" ]]; then
            __intent_sh_binding_state=ours
        else
            case $line in
                "ble-bind -m '$__intent_sh_blesh_keymap' -f $key "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -f '$key' "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -x $key "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -x '$key' "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -c $key "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -c '$key' "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -s $key "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -s '$key' "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -@ $key "*|\
                "ble-bind -m '$__intent_sh_blesh_keymap' -@ '$key' "*)
                    __intent_sh_binding_state=conflict
                    ;;
            esac
        fi
    done < <(ble-bind -P -m "$__intent_sh_blesh_keymap" 2>/dev/null)
    [[ $__intent_sh_binding_state != conflict ]]
}

__intent_sh_restore_blesh_binding() {
    local key=$1 default_widget=$2 state=$3
    case $state in
        default)
            ble-bind -m "$__intent_sh_blesh_keymap" -f "$key" "$default_widget" >/dev/null 2>&1 || :
            ;;
        unbound)
            ble-bind -m "$__intent_sh_blesh_keymap" -f "$key" - >/dev/null 2>&1 || :
            ;;
    esac
}

__intent_sh_initialize_blesh() {
    __intent_sh_editor_backend=blesh
    __intent_sh_editor_version=$__intent_sh_expected_blesh_version
    if ! __intent_sh_require_blesh_api; then
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" missing_api "" \
            "the attached ble.sh is missing a required edit or widget API; load the tested version and try again"
        return 1
    fi

    __intent_sh_blesh_keymap=${_ble_decode_keymap-}
    case $__intent_sh_blesh_keymap in
        emacs|vi_imap) ;;
        *)
            __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" unsupported_keymap "" \
                "ble.sh must be in Emacs or Vi insert mode before intent-sh is initialized"
            return 1
            ;;
    esac

    local mg_state mu_state
    if ! __intent_sh_blesh_binding_state M-g 'complete context=glob' __intent_sh_rewrite; then
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_conflict M-g \
            "ble.sh already has an unsupported M-g binding; resolve it before initializing intent-sh"
        return 1
    fi
    mg_state=$__intent_sh_binding_state
    if ! __intent_sh_blesh_binding_state M-u upcase-eword __intent_sh_undo; then
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_conflict M-u \
            "ble.sh already has an unsupported M-u binding; resolve it before initializing intent-sh"
        return 1
    fi
    mu_state=$__intent_sh_binding_state

    if declare -F ble/function#advice/around:ble/widget/default/accept-line >/dev/null 2>&1 &&
        [[ $__intent_sh_blesh_advice_installed != 1 ]]
    then
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_conflict accept-line \
            "ble.sh already has an accept-line advice; resolve it before initializing intent-sh"
        return 1
    fi

    if ! ble-bind -m "$__intent_sh_blesh_keymap" -x M-g __intent_sh_rewrite >/dev/null 2>&1; then
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_failed M-g \
            "ble.sh could not install the M-g rewrite binding"
        return 1
    fi
    if ! ble-bind -m "$__intent_sh_blesh_keymap" -x M-u __intent_sh_undo >/dev/null 2>&1; then
        __intent_sh_restore_blesh_binding M-g 'complete context=glob' "$mg_state"
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_failed M-u \
            "ble.sh could not install the M-u undo binding"
        return 1
    fi
    if ! ble/function#advice around ble/widget/default/accept-line \
        '__intent_sh_blesh_accept_guard; [[ $__intent_sh_blesh_accept_action != delegate ]] || ble/function#advice/do' \
        >/dev/null 2>&1; then
        __intent_sh_restore_blesh_binding M-g 'complete context=glob' "$mg_state"
        __intent_sh_restore_blesh_binding M-u upcase-eword "$mu_state"
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_failed accept-line \
            "ble.sh could not install the guarded accept-line integration"
        return 1
    fi
    __intent_sh_blesh_advice_installed=1
    if ! blehook 'DETACH!=__intent_sh_blesh_detach' >/dev/null 2>&1; then
        ble/function#advice remove ble/widget/default/accept-line >/dev/null 2>&1 || :
        __intent_sh_blesh_advice_installed=
        __intent_sh_restore_blesh_binding M-g 'complete context=glob' "$mg_state"
        __intent_sh_restore_blesh_binding M-u upcase-eword "$mu_state"
        __intent_sh_fail_initialization blesh "$__intent_sh_editor_version" binding_failed detach-hook \
            "ble.sh could not install the detach safety hook"
        return 1
    fi

    __intent_sh_set_status blesh "$__intent_sh_editor_version" 1 "" ""
    return 0
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

    if ((BASH_VERSINFO[0] < 3 || BASH_VERSINFO[0] == 3 && BASH_VERSINFO[1] < 2)); then
        __intent_sh_fail_initialization none none unsupported_bash "" \
            "Bash 3.2 is the conditional minimum; use stock Zsh or install a modern Bash"
        return 1
    fi

    if [[ ${BLE_ATTACHED-} == 1 ]]; then
        if [[ $__intent_sh_rewrite_key != alt+g || $__intent_sh_undo_key != alt+u ]]; then
            __intent_sh_fail_initialization blesh "${BLE_VERSION-unknown}" unsupported_binding "" \
                "custom rewrite and undo bindings currently require native Bash 4+ Readline or Zsh; the existing ble.sh contract keeps Alt+G and Alt+U"
            return 1
        fi
        if [[ ${BLE_VERSION-} != "$__intent_sh_expected_blesh_version" ]]; then
            __intent_sh_fail_initialization blesh unsupported incompatible_version "" \
                "attached ble.sh is untested; load $__intent_sh_expected_blesh_version before intent-sh"
            return 1
        fi
        __intent_sh_initialize_blesh
        return $?
    fi

    if ((BASH_VERSINFO[0] >= 4)); then
        __intent_sh_initialize_readline
        return $?
    fi

    if [[ ${BLE_VERSION-} == "$__intent_sh_expected_blesh_version" ]] || declare -F ble-bind >/dev/null 2>&1; then
        __intent_sh_fail_initialization blesh "$__intent_sh_expected_blesh_version" not_attached "" \
            "ble.sh is loaded but not attached; attach it before evaluating intent-sh init bash"
    else
        __intent_sh_fail_initialization none none missing_blesh "" \
            "Bash 3.2 requires the tested ble.sh loaded first; alternatively use stock Zsh or install Bash 4+"
    fi
    return 1
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
