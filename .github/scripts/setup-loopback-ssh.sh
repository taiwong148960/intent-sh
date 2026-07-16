#!/usr/bin/env bash

set -euo pipefail
umask 077

action=${1-}
if [[ $# -ne 1 || ( $action != start && $action != stop ) ]]; then
  printf 'usage: %s start|stop\n' "$0" >&2
  exit 2
fi

runner_temp=${RUNNER_TEMP-}
if [[ ! $runner_temp =~ ^/[A-Za-z0-9._/+@%-]{1,400}$ || $runner_temp == / || $runner_temp == *'/../'* || $runner_temp == *'/./'* ]]; then
  printf 'RUNNER_TEMP must be one bounded absolute CI-owned path\n' >&2
  exit 1
fi

state_root=$runner_temp/intent-sh-loopback-ssh
state_file=$state_root/state
expected_client_config=$state_root/client_config
expected_sshd_config=$state_root/sshd_config
expected_pid_file=$state_root/sshd.pid

account=
home=
port=
sshd_pid=0

validate_account() {
  [[ $account =~ ^intentci[0-9]{1,12}a[0-9]{1,3}$ ]]
}

validate_home() {
  [[ $home == "/tmp/intent-sh-loopback-ssh-$account" ]]
}

write_state() {
  local temporary=$state_file.new
  {
    printf 'ACCOUNT=%s\n' "$account"
    printf 'HOME=%s\n' "$home"
    printf 'PORT=%s\n' "$port"
    printf 'SSHD_PID=%s\n' "$sshd_pid"
    printf 'SSHD_CONFIG=%s\n' "$expected_sshd_config"
    printf 'CLIENT_CONFIG=%s\n' "$expected_client_config"
  } > "$temporary"
  chmod 600 "$temporary"
  mv -f -- "$temporary" "$state_file"
}

load_state() {
  local key value
  [[ -f $state_file && ! -L $state_file ]] || return 1
  while IFS='=' read -r key value; do
    case "$key" in
      ACCOUNT) account=$value ;;
      HOME) home=$value ;;
      PORT) port=$value ;;
      SSHD_PID) sshd_pid=$value ;;
      SSHD_CONFIG) [[ $value == "$expected_sshd_config" ]] || return 1 ;;
      CLIENT_CONFIG) [[ $value == "$expected_client_config" ]] || return 1 ;;
      *) return 1 ;;
    esac
  done < "$state_file"
  validate_account || return 1
  validate_home || return 1
  [[ $port =~ ^[0-9]{5}$ ]] && (( port >= 20000 && port <= 60999 )) || return 1
  [[ $sshd_pid =~ ^[0-9]{1,10}$ ]] || return 1
}

stop_server() {
  local failure=0
  if [[ ! -e $state_root ]]; then
    printf 'loopback SSH state is already absent\n'
    return 0
  fi
  if [[ -L $state_root || ! -d $state_root ]] || ! load_state; then
    printf 'refusing to clean invalid loopback SSH state\n' >&2
    return 1
  fi

  if (( sshd_pid > 1 )) && sudo -n kill -0 "$sshd_pid" 2>/dev/null; then
    if [[ -r /proc/$sshd_pid/cmdline ]] && ! tr '\0' ' ' < "/proc/$sshd_pid/cmdline" | grep -F -q -- "$expected_sshd_config"; then
      printf 'refusing to stop a PID that is not the job-owned sshd\n' >&2
      return 1
    fi
    sudo -n kill "$sshd_pid"
    for _ in {1..50}; do
      sudo -n kill -0 "$sshd_pid" 2>/dev/null || break
      sleep 0.1
    done
    if sudo -n kill -0 "$sshd_pid" 2>/dev/null; then
      sudo -n kill -KILL "$sshd_pid"
      for _ in {1..50}; do
        sudo -n kill -0 "$sshd_pid" 2>/dev/null || break
        sleep 0.1
      done
      if sudo -n kill -0 "$sshd_pid" 2>/dev/null; then
        printf 'job-owned sshd did not terminate; retaining state for recovery\n' >&2
        return 1
      fi
    fi
  fi

  if getent passwd "$account" >/dev/null; then
    actual_home=$(getent passwd "$account" | awk -F: '{print $6}')
    if [[ $actual_home != "$home" ]]; then
      printf 'refusing to remove an account with unexpected home state\n' >&2
      return 1
    fi
    sudo -n pkill -KILL -u "$account" 2>/dev/null || true
    for _ in {1..50}; do
      sudo -n pgrep -u "$account" >/dev/null 2>&1 || break
      sleep 0.1
    done
    if sudo -n pgrep -u "$account" >/dev/null 2>&1; then
      printf 'job-owned account still has processes; retaining state for recovery\n' >&2
      return 1
    fi
    sudo -n userdel "$account" || failure=1
  fi
  sudo -n rm -rf -- "$home"
  rm -rf -- "$state_root"

  if getent passwd "$account" >/dev/null || [[ -e $home || -e $state_root ]]; then
    printf 'loopback SSH cleanup left job-owned state behind\n' >&2
    failure=1
  fi
  (( failure == 0 ))
}

if [[ $action == stop ]]; then
  stop_server
  exit
fi

if [[ $(uname -s) != Linux ]]; then
  printf 'the required loopback SSH fixture is Linux-only\n' >&2
  exit 1
fi
for command in awk getent pgrep pkill ssh ssh-keygen sudo tr; do
  command -v "$command" >/dev/null || {
    printf 'required loopback SSH command is missing: %s\n' "$command" >&2
    exit 1
  }
done
sshd_path=$(command -v sshd || true)
if [[ -z $sshd_path && -x /usr/sbin/sshd ]]; then
  sshd_path=/usr/sbin/sshd
fi
if [[ ! $sshd_path =~ ^/[A-Za-z0-9._/+@%-]{1,400}$ || ! -x $sshd_path ]]; then
  printf 'a bounded absolute sshd executable is required\n' >&2
  exit 1
fi
sudo -n true

if [[ -e $state_root ]]; then
  printf 'loopback SSH state already exists; run stop before start\n' >&2
  exit 1
fi
mkdir -m 700 -- "$state_root"

run_id=${GITHUB_RUN_ID:-$$}
attempt=${GITHUB_RUN_ATTEMPT:-1}
[[ $run_id =~ ^[0-9]{1,20}$ && $attempt =~ ^[0-9]{1,3}$ ]] || {
  printf 'GitHub run metadata must be numeric\n' >&2
  exit 1
}
account="intentci$((run_id % 1000000000))a$attempt"
validate_account || {
  printf 'derived loopback account was invalid\n' >&2
  exit 1
}
home=/tmp/intent-sh-loopback-ssh-$account
validate_home
port=$((30000 + (run_id + attempt) % 20000))

trap 'status=$?; if (( status != 0 )); then stop_server || true; fi' EXIT

if getent passwd "$account" >/dev/null || [[ -e $home ]]; then
  printf 'derived loopback account or home already exists\n' >&2
  exit 1
fi
sudo -n useradd --create-home --home-dir "$home" --shell /bin/bash "$account"
sudo -n passwd -d "$account" >/dev/null
write_state

host_key=$state_root/host_ed25519
client_key=$state_root/client_ed25519
known_hosts=$state_root/known_hosts
sshd_log=$state_root/sshd.log
ssh-keygen -q -t ed25519 -N '' -C intent-sh-ci-host -f "$host_key"
ssh-keygen -q -t ed25519 -N '' -C intent-sh-ci-client -f "$client_key"

sudo -n install -d -m 700 -o "$account" -g "$account" "$home/.ssh"
{
  printf 'no-agent-forwarding,no-port-forwarding,no-X11-forwarding,no-user-rc '
  cat "$client_key.pub"
} | sudo -n tee "$home/.ssh/authorized_keys" >/dev/null
sudo -n chown "$account:$account" "$home/.ssh/authorized_keys"
sudo -n chmod 600 "$home/.ssh/authorized_keys"

read -r host_key_type host_key_data _ < "$host_key.pub"
printf '[127.0.0.1]:%s %s %s\n' "$port" "$host_key_type" "$host_key_data" > "$known_hosts"
chmod 600 "$known_hosts"

{
  printf 'Port %s\n' "$port"
  printf 'ListenAddress 127.0.0.1\n'
  printf 'AddressFamily inet\n'
  printf 'HostKey %s\n' "$host_key"
  printf 'PidFile %s\n' "$expected_pid_file"
  printf 'AuthorizedKeysFile .ssh/authorized_keys\n'
  printf 'AllowUsers %s\n' "$account"
  printf 'AuthenticationMethods publickey\n'
  printf 'PubkeyAuthentication yes\n'
  printf 'PasswordAuthentication no\n'
  printf 'KbdInteractiveAuthentication no\n'
  printf 'ChallengeResponseAuthentication no\n'
  printf 'PermitEmptyPasswords no\n'
  printf 'UsePAM no\n'
  printf 'PermitRootLogin no\n'
  printf 'AllowAgentForwarding no\n'
  printf 'AllowTcpForwarding no\n'
  printf 'GatewayPorts no\n'
  printf 'X11Forwarding no\n'
  printf 'PermitTunnel no\n'
  printf 'PermitUserEnvironment no\n'
  printf 'PermitUserRC no\n'
  printf 'StrictModes yes\n'
  printf 'UseDNS no\n'
  printf 'PrintMotd no\n'
  printf 'LogLevel VERBOSE\n'
  printf 'Subsystem sftp internal-sftp\n'
} > "$expected_sshd_config"
chmod 600 "$expected_sshd_config"

{
  printf 'Host intent-sh-loopback\n'
  printf '  HostName 127.0.0.1\n'
  printf '  Port %s\n' "$port"
  printf '  User %s\n' "$account"
  printf '  IdentityFile %s\n' "$client_key"
  printf '  UserKnownHostsFile %s\n' "$known_hosts"
  printf '  IdentitiesOnly yes\n'
  printf '  BatchMode yes\n'
  printf '  PasswordAuthentication no\n'
  printf '  KbdInteractiveAuthentication no\n'
  printf '  StrictHostKeyChecking yes\n'
  printf '  UpdateHostKeys no\n'
  printf '  ClearAllForwardings yes\n'
  printf '  ForwardAgent no\n'
  printf '  ForwardX11 no\n'
  printf '  PermitLocalCommand no\n'
  printf '  RequestTTY no\n'
  printf '  ConnectTimeout 10\n'
  printf '  LogLevel ERROR\n'
} > "$expected_client_config"
chmod 600 "$expected_client_config" "$client_key"

sudo -n mkdir -p /run/sshd
sudo -n "$sshd_path" -t -f "$expected_sshd_config"
sudo -n "$sshd_path" -f "$expected_sshd_config" -E "$sshd_log"
for _ in {1..50}; do
  [[ -s $expected_pid_file ]] && break
  sleep 0.1
done
[[ -s $expected_pid_file ]] || {
  printf 'loopback sshd did not publish its PID\n' >&2
  exit 1
}
read -r sshd_pid < "$expected_pid_file"
[[ $sshd_pid =~ ^[0-9]{1,10}$ ]] || {
  printf 'loopback sshd published an invalid PID\n' >&2
  exit 1
}
write_state

ready=0
for _ in {1..50}; do
  if ssh -F "$expected_client_config" -o BatchMode=yes -o ClearAllForwardings=yes -T intent-sh-loopback true 2>/dev/null; then
    ready=1
    break
  fi
  sleep 0.1
done
(( ready == 1 )) || {
  printf 'loopback SSH alias did not become ready\n' >&2
  exit 1
}

if [[ -n ${GITHUB_ENV-} ]]; then
  [[ $GITHUB_ENV =~ ^/[A-Za-z0-9._/+@%-]{1,500}$ && ! -L $GITHUB_ENV ]] || {
    printf 'GITHUB_ENV must be one bounded regular CI environment path\n' >&2
    exit 1
  }
  {
    printf 'INTENT_SH_TEST_SSH_TARGET=intent-sh-loopback\n'
    printf 'INTENT_SH_TEST_SSH_CONFIG=%s\n' "$expected_client_config"
    printf 'INTENT_SH_TEST_SSH_LOOPBACK=1\n'
  } >> "$GITHUB_ENV"
else
  printf 'INTENT_SH_TEST_SSH_TARGET=intent-sh-loopback\n'
  printf 'INTENT_SH_TEST_SSH_CONFIG=%s\n' "$expected_client_config"
fi

trap - EXIT
printf 'loopback SSH fixture ready on a high localhost port\n'
