package shelltest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

const promptMarker = "__INTENT_PROMPT__> "

type shellCase struct {
	name       string
	executable string
	args       []string
	script     string
}

func TestDangerousConfirmationInPTY(t *testing.T) {
	root := repositoryRoot(t)
	cases := []shellCase{
		{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
		{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell := startShell(t, tc)
			defer shell.close(t)

			marker := filepath.Join(t.TempDir(), "danger-ran")
			command := "touch " + shellQuote(marker)
			shell.configureDanger(t, command)
			shell.write(t, command)
			shell.writeBytes(t, []byte{'\r'})
			output := shell.readUntil(t, "Press Enter again to execute.")
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("first Enter executed target; output=%q stat=%v", output, err)
			}

			shell.writeBytes(t, []byte{'\r'})
			waitForPath(t, marker, shell)
		})
	}
}

func TestOrdinaryCommandUsesOneEnter(t *testing.T) {
	root := repositoryRoot(t)
	cases := []shellCase{
		{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
		{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell := startShell(t, tc)
			defer shell.close(t)
			marker := filepath.Join(t.TempDir(), "safe-ran")
			shell.write(t, "touch "+shellQuote(marker))
			shell.writeBytes(t, []byte{'\r'})
			waitForPath(t, marker, shell)
		})
	}
}

func TestEditingDisarmsDangerousFingerprint(t *testing.T) {
	root := repositoryRoot(t)
	cases := []shellCase{
		{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
		{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell := startShell(t, tc)
			defer shell.close(t)
			marker := filepath.Join(t.TempDir(), "edited-ran")
			command := "touch " + shellQuote(marker)
			shell.configureDanger(t, command)
			shell.write(t, command)
			shell.writeBytes(t, []byte{'\r'})
			shell.readUntil(t, "Press Enter again to execute.")
			shell.write(t, " ")
			shell.writeBytes(t, []byte{'\r'})
			waitForPath(t, marker, shell)
		})
	}
}

func TestBash32WithoutBleshFailsBeforeBinding(t *testing.T) {
	path, err := exec.LookPath("/bin/bash")
	if err != nil {
		t.Skip("system Bash is not available")
	}
	if major := bashMajor(t, path); major >= 4 {
		t.Skipf("system Bash is %d, not an incompatible version", major)
	}
	script := filepath.Join(repositoryRoot(t), "shell", "bash", "intent-sh.bash")
	command := fmt.Sprintf("source %s; status=$?; bind -s; printf 'STATUS:%%s BACKEND:%%s READY:%%s FAILURE:%%s\\n' \"$status\" \"$INTENT_SH_ADAPTER_BACKEND\" \"$INTENT_SH_ADAPTER_READY\" \"$INTENT_SH_ADAPTER_FAILURE\"", shellQuote(script))
	cmd := exec.Command(path, "--noprofile", "--norc", "-ic", command)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("run incompatible Bash probe: %v: %s", runErr, output)
	}
	text := string(output)
	if !strings.Contains(text, "Bash 3.2 requires the tested ble.sh loaded first") ||
		!strings.Contains(text, "STATUS:1 BACKEND:none READY:0 FAILURE:missing_blesh") {
		t.Fatalf("incompatible Bash did not fail clearly: %q", text)
	}
	if strings.Contains(text, `"\C-m":"\C-]\C-^"`) {
		t.Fatalf("incompatible Bash installed Enter binding: %q", text)
	}
}

type runningShell struct {
	cmd                    *exec.Cmd
	file                   *os.File
	chunks                 chan []byte
	readErr                chan error
	pending                string
	name                   string
	respondTerminalQueries bool
}

func startShell(t *testing.T, tc shellCase) *runningShell {
	return startShellWith(t, tc, nil, "source "+shellQuote(tc.script))
}

func startShellWith(t *testing.T, tc shellCase, extraEnv map[string]string, initialize string) *runningShell {
	return startShellWithOptions(t, tc, extraEnv, initialize, false)
}

func startShellWithTerminalResponses(t *testing.T, tc shellCase, extraEnv map[string]string, initialize string) *runningShell {
	return startShellWithOptions(t, tc, extraEnv, initialize, true)
}

func startBashWithRCAndTerminalResponses(t *testing.T, tc shellCase, extraEnv map[string]string, bashrc string) *runningShell {
	t.Helper()
	rcPath := filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(rcPath, []byte(bashrc+"\n"), 0o600); err != nil {
		t.Fatalf("write Bash test rcfile: %v", err)
	}
	tc.args = []string{"--noprofile", "--rcfile", rcPath, "-i"}
	return startShellWithOptions(t, tc, extraEnv, "", true)
}

func startShellWithOptions(t *testing.T, tc shellCase, extraEnv map[string]string, initialize string, respondTerminalQueries bool) *runningShell {
	t.Helper()
	path, err := exec.LookPath(tc.executable)
	if err != nil {
		t.Skipf("%s is not installed", tc.executable)
	}
	cmd := exec.Command(path, tc.args...)
	environment := map[string]string{"PS1": promptMarker, "PROMPT": promptMarker, "TERM": "dumb"}
	for key, value := range extraEnv {
		environment[key] = value
	}
	cmd.Env = replaceEnvironment(os.Environ(), environment)
	file, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start %s: %v", tc.name, err)
	}
	_ = pty.Setsize(file, &pty.Winsize{Rows: 40, Cols: 240})
	shell := &runningShell{cmd: cmd, file: file, chunks: make(chan []byte, 32), readErr: make(chan error, 1), name: tc.name, respondTerminalQueries: respondTerminalQueries}
	go shell.readLoop()
	shell.readUntil(t, promptMarker)
	if initialize != "" {
		shell.write(t, initialize)
		shell.writeBytes(t, []byte{'\r'})
		shell.readUntil(t, promptMarker)
	}
	return shell
}

func replaceEnvironment(source []string, replacements map[string]string) []string {
	result := make([]string, 0, len(source)+len(replacements))
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replace := replacements[key]; replace {
				continue
			}
		}
		result = append(result, entry)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}

func (s *runningShell) configureDanger(t *testing.T, command string) {
	t.Helper()
	setup := fmt.Sprintf("__intent_sh_generated_command=%s; __intent_sh_risk=dangerous; __intent_sh_risk_reason='test danger'; __intent_sh_armed_fingerprint=''", shellQuote(command))
	s.write(t, setup)
	s.writeBytes(t, []byte{'\r'})
	s.readUntil(t, promptMarker)
}

func (s *runningShell) write(t *testing.T, value string) {
	t.Helper()
	s.writeBytes(t, []byte(value))
}

func (s *runningShell) writeBytes(t *testing.T, value []byte) {
	t.Helper()
	if _, err := s.file.Write(value); err != nil {
		t.Fatalf("write PTY: %v", err)
	}
}

func (s *runningShell) readUntil(t *testing.T, needle string) string {
	return s.readUntilTimeout(t, needle, 5*time.Second)
}

func (s *runningShell) readUntilTimeout(t *testing.T, needle string, timeout time.Duration) string {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if index := strings.Index(s.pending, needle); index >= 0 {
			end := index + len(needle)
			matched := s.pending[:end]
			s.pending = s.pending[end:]
			return matched
		}
		select {
		case chunk := <-s.chunks:
			s.pending += string(chunk)
		case err := <-s.readErr:
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				t.Fatalf("read PTY waiting for %q: %v; output=%q", needle, err, s.pending)
			}
			t.Fatalf("shell exited waiting for %q; output=%q", needle, s.pending)
		case <-timer.C:
			t.Fatalf("timed out waiting for %q; output=%q", needle, s.pending)
		}
	}
}

func (s *runningShell) readLoop() {
	buffer := make([]byte, 1024)
	var terminalQueryTail []byte
	cprCount := 0
	for {
		n, err := s.file.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			if s.respondTerminalQueries {
				scan := append(append([]byte(nil), terminalQueryTail...), chunk...)
				for _, query := range []struct {
					request  []byte
					response []byte
				}{
					{[]byte("\x1b[>c"), []byte("\x1b[>0;95;0c")},
					{[]byte("\x1b[c"), []byte("\x1b[?1;2c")},
				} {
					for range bytes.Count(scan, query.request) {
						_, _ = s.file.Write(query.response)
					}
				}
				for range bytes.Count(scan, []byte("\x1b[6n")) {
					cprCount++
					response := []byte("\x1b[1;1R")
					if cprCount == 1 {
						response = []byte("\x1b[2;1R")
					} else if cprCount == 2 {
						response = []byte("\x1b[3;1R")
					}
					_, _ = s.file.Write(response)
				}
				const queryPrefixLimit = 3
				if len(scan) > queryPrefixLimit {
					terminalQueryTail = append(terminalQueryTail[:0], scan[len(scan)-queryPrefixLimit:]...)
				} else {
					terminalQueryTail = append(terminalQueryTail[:0], scan...)
				}
			}
			s.chunks <- chunk
		}
		if err != nil {
			s.readErr <- err
			return
		}
	}
}

func (s *runningShell) close(t *testing.T) {
	t.Helper()
	_, _ = s.file.Write([]byte("exit\r"))
	_ = s.file.Close()
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func testBash() string {
	if path := os.Getenv("INTENT_SH_TEST_BASH"); path != "" {
		return path
	}
	return "bash"
}

func requireCompatibleShell(t *testing.T, tc shellCase) {
	t.Helper()
	if tc.name == "bash" {
		path, err := exec.LookPath(tc.executable)
		if err != nil {
			t.Skipf("%s is not installed", tc.executable)
		}
		if major := bashMajor(t, path); major < 4 {
			t.Skipf("Bash %d is intentionally incompatible; set INTENT_SH_TEST_BASH to Bash 4.0+", major)
		}
	}
}

func bashMajor(t *testing.T, executable string) int {
	t.Helper()
	output, err := exec.Command(executable, "-c", `printf '%s' "${BASH_VERSINFO[0]}"`).Output()
	if err != nil {
		t.Fatalf("inspect Bash version: %v", err)
	}
	var major int
	if _, err := fmt.Sscanf(string(output), "%d", &major); err != nil {
		t.Fatalf("parse Bash version %q: %v", output, err)
	}
	return major
}
