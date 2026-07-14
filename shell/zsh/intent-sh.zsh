# intent-sh Zsh adapter (protocol 2)
# Loaded through: eval "$(intent-sh init zsh)"

[[ -o interactive ]] || return 0

typeset __intent_sh_requested_rewrite_key=__INTENT_SH_REWRITE_CANONICAL__
typeset __intent_sh_requested_undo_key=__INTENT_SH_UNDO_CANONICAL__
typeset __intent_sh_requested_rewrite_binding=$'__INTENT_SH_REWRITE_BINDING__'
typeset __intent_sh_requested_undo_binding=$'__INTENT_SH_UNDO_BINDING__'
[[ $__intent_sh_requested_rewrite_key == __INTENT_SH_""REWRITE_CANONICAL__ ]] && __intent_sh_requested_rewrite_key=alt+g
[[ $__intent_sh_requested_undo_key == __INTENT_SH_""UNDO_CANONICAL__ ]] && __intent_sh_requested_undo_key=alt+u
[[ $__intent_sh_requested_rewrite_binding == __INTENT_SH_""REWRITE_BINDING__ ]] && __intent_sh_requested_rewrite_binding=$'\x1b\x67'
[[ $__intent_sh_requested_undo_binding == __INTENT_SH_""UNDO_BINDING__ ]] && __intent_sh_requested_undo_binding=$'\x1b\x75'

if (( ${+__intent_sh_loaded} )); then
    if [[ ${__intent_sh_protocol_version-} != 2 ]]; then
        print -u2 -- "intent-sh: loaded adapter protocol ${__intent_sh_protocol_version-unknown} is incompatible with protocol 2"
        return 1
    fi
    if [[ ${__intent_sh_rewrite_key-} != "$__intent_sh_requested_rewrite_key" || ${__intent_sh_undo_key-} != "$__intent_sh_requested_undo_key" ]]; then
        print -u2 -- "intent-sh: different rewrite or undo bindings are already active; start a new shell before loading the new configuration"
        return 1
    fi
    return 0
fi

typeset -g __intent_sh_loaded=1
typeset -g __intent_sh_protocol_version=2
typeset -g __intent_sh_editor_backend=zle
typeset -g __intent_sh_editor_version=$ZSH_VERSION
typeset -g __intent_sh_rewrite_key=$__intent_sh_requested_rewrite_key
typeset -g __intent_sh_undo_key=$__intent_sh_requested_undo_key
typeset -g __intent_sh_original_buffer=
typeset -gi __intent_sh_original_cursor=0
typeset -g __intent_sh_generated_command=
typeset -g __intent_sh_provider=
typeset -g __intent_sh_risk=
typeset -g __intent_sh_risk_reason=
typeset -g __intent_sh_request_id=
typeset -g __intent_sh_active_request_id=
typeset -gi __intent_sh_generation_index=0
typeset -gi __intent_sh_request_counter=0
typeset -g __intent_sh_armed_fingerprint=

function __intent_sh_clear_chain() {
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

function __intent_sh_message() {
    zle -M -- "intent-sh: $1"
}

function __intent_sh_cursor_to_protocol_bytes() {
    local text=$1 editor_cursor=$2 prefix=
    (( editor_cursor > 0 )) && prefix=${text[1,editor_cursor]}
    local LC_ALL=C
    typeset -g __intent_sh_protocol_cursor_bytes=${#prefix}
}

function __intent_sh_read_response() {
    local path=$1
    local parse_ok=1 extra=
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
    (( parse_ok ))
}

function __intent_sh_rewrite() {
    setopt localtraps
    local current=$BUFFER
    local current_cursor=$CURSOR
    __intent_sh_cursor_to_protocol_bytes "$current" "$current_cursor"
    local current_protocol_cursor=$__intent_sh_protocol_cursor_bytes
    local request_original= request_previous=
    local request_generation=0 pending_original=$current
    local pending_original_cursor=$current_cursor

    __intent_sh_armed_fingerprint=
    if [[ -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" && -n $__intent_sh_original_buffer ]]; then
        request_original=$__intent_sh_original_buffer
        request_previous=$__intent_sh_generated_command
        request_generation=$(( __intent_sh_generation_index + 1 ))
        pending_original=$__intent_sh_original_buffer
        pending_original_cursor=$__intent_sh_original_cursor
    else
        __intent_sh_clear_chain
    fi

    if [[ -z ${current//[[:space:]]/} ]]; then
        __intent_sh_message "enter a command or intent before requesting a rewrite"
        return 0
    fi

    if ! (( $+commands[intent-sh] )); then
        __intent_sh_message "binary not found on PATH"
        return 0
    fi

    __intent_sh_request_counter=$(( __intent_sh_request_counter + 1 ))
    local request_id="zsh-$$-${__intent_sh_request_counter}-${RANDOM}"
    __intent_sh_active_request_id=$request_id
    local tmp
    tmp=$(mktemp "${TMPDIR:-/tmp}/intent-sh-zsh.XXXXXXXX") || {
        __intent_sh_active_request_id=
        __intent_sh_message "could not create a temporary response file"
        return 0
    }

    local __intent_sh_interrupted=0
    local __intent_sh_provider_pid=
    trap '__intent_sh_interrupted=1; [[ -n $__intent_sh_provider_pid ]] && kill -INT "$__intent_sh_provider_pid" 2>/dev/null' INT
    local command_status=0
    { printf '%s\0' \
            "$__intent_sh_protocol_version" \
            rewrite \
            zsh \
            "$ZSH_VERSION" \
            "$__intent_sh_editor_backend" \
            "$__intent_sh_editor_version" \
            "$current" \
            "$current_protocol_cursor" \
            "$request_original" \
            "$request_previous" \
            "$request_generation" \
            "$request_id"
    } | command intent-sh adapter rewrite --protocol "$__intent_sh_protocol_version" >| "$tmp" &
    __intent_sh_provider_pid=$!
    __intent_sh_message "generating… (Ctrl+C to cancel)"
    zle -R
    wait "$__intent_sh_provider_pid" || command_status=$?
    if (( __intent_sh_interrupted )); then
        kill -INT "$__intent_sh_provider_pid" 2>/dev/null
        wait "$__intent_sh_provider_pid" 2>/dev/null
        command_status=130
    fi
    trap - INT

    if ! __intent_sh_read_response "$tmp"; then
        command rm -f -- "$tmp"
        __intent_sh_active_request_id=
        BUFFER=$current
        CURSOR=$current_cursor
        if (( __intent_sh_interrupted )); then
            __intent_sh_message "cancelled"
        else
            __intent_sh_message "received a malformed adapter response"
        fi
        return 0
    fi
    command rm -f -- "$tmp"

    if [[ $__intent_sh_response_version != $__intent_sh_protocol_version ]]; then
        __intent_sh_active_request_id=
        BUFFER=$current
        CURSOR=$current_cursor
        __intent_sh_message "adapter protocol mismatch"
        return 0
    fi
    if [[ $__intent_sh_response_request_id != $request_id || $__intent_sh_active_request_id != $request_id ]]; then
        __intent_sh_active_request_id=
        BUFFER=$current
        CURSOR=$current_cursor
        __intent_sh_message "ignored a stale adapter response"
        return 0
    fi
    __intent_sh_active_request_id=

    case $__intent_sh_response_status in
        ok)
            if [[ -z $__intent_sh_response_replacement ]]; then
                BUFFER=$current
                CURSOR=$current_cursor
                __intent_sh_message "adapter returned an empty replacement"
                return 0
            fi
            BUFFER=$__intent_sh_response_replacement
            CURSOR=${#BUFFER}
            __intent_sh_original_buffer=$pending_original
            __intent_sh_original_cursor=$pending_original_cursor
            __intent_sh_generated_command=$BUFFER
            __intent_sh_provider=$__intent_sh_response_provider
            __intent_sh_risk=$__intent_sh_response_risk
            __intent_sh_risk_reason=$__intent_sh_response_risk_reason
            __intent_sh_request_id=$request_id
            __intent_sh_generation_index=$request_generation
            __intent_sh_armed_fingerprint=
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
            BUFFER=$current
            CURSOR=$current_cursor
            __intent_sh_message "${__intent_sh_response_message:-more detail is required}"
            ;;
        cancelled)
            BUFFER=$current
            CURSOR=$current_cursor
            __intent_sh_message "cancelled"
            ;;
        error)
            BUFFER=$current
            CURSOR=$current_cursor
            __intent_sh_message "${__intent_sh_response_message:-rewrite failed}"
            ;;
        *)
            BUFFER=$current
            CURSOR=$current_cursor
            __intent_sh_message "adapter returned an unknown status (exit $command_status)"
            ;;
    esac
    zle -R
    return 0
}

function __intent_sh_undo() {
    if [[ -n $__intent_sh_generated_command && $BUFFER == "$__intent_sh_generated_command" && -n $__intent_sh_original_buffer ]]; then
        local restored=$__intent_sh_original_buffer
        local restored_cursor=$__intent_sh_original_cursor
        __intent_sh_clear_chain
        BUFFER=$restored
        CURSOR=$restored_cursor
        __intent_sh_message "restored the original buffer"
        return 0
    fi
    if [[ -n $__intent_sh_generated_command && $BUFFER != "$__intent_sh_generated_command" ]]; then
        __intent_sh_clear_chain
        __intent_sh_message "buffer was edited; undo did not overwrite it"
        return 0
    fi
    __intent_sh_message "nothing to restore"
}

function __intent_sh_accept_line() {
    local current=$BUFFER
    if [[ -n $__intent_sh_generated_command && $current != "$__intent_sh_generated_command" ]]; then
        __intent_sh_clear_chain
        zle .accept-line
        return
    fi
    if [[ $__intent_sh_risk == dangerous && -n $__intent_sh_generated_command && $current == "$__intent_sh_generated_command" ]]; then
        if [[ $__intent_sh_armed_fingerprint == "$current" ]]; then
            __intent_sh_clear_chain
            zle .accept-line
            return
        fi
        __intent_sh_armed_fingerprint=$current
        __intent_sh_message "DANGEROUS: ${__intent_sh_risk_reason:-dangerous command}. Press Enter again to execute."
        return
    fi
    __intent_sh_clear_chain
    zle .accept-line
}

zle -N intent-sh-rewrite __intent_sh_rewrite
zle -N intent-sh-undo __intent_sh_undo
zle -N intent-sh-accept-line __intent_sh_accept_line
bindkey "$__intent_sh_requested_rewrite_binding" intent-sh-rewrite
bindkey "$__intent_sh_requested_undo_binding" intent-sh-undo
bindkey '^M' intent-sh-accept-line
bindkey '^J' intent-sh-accept-line

# Export only bounded capability markers for child diagnostics. Buffer,
# generated-command, and binding-body state remains private to this shell.
typeset -gx INTENT_SH_ADAPTER_PROTOCOL=2
typeset -gx INTENT_SH_ADAPTER_BACKEND=zle
typeset -gx INTENT_SH_ADAPTER_EDITOR_VERSION=$ZSH_VERSION
typeset -gx INTENT_SH_ADAPTER_READY=1
typeset -gx INTENT_SH_ADAPTER_FAILURE=
typeset -gx INTENT_SH_ADAPTER_CONFLICTS=
typeset -gx INTENT_SH_ADAPTER_REWRITE_KEY=$__intent_sh_rewrite_key
typeset -gx INTENT_SH_ADAPTER_UNDO_KEY=$__intent_sh_undo_key
