package setup

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInspectSelectsLikelyStartupFile(t *testing.T) {
	t.Parallel()
	home := "/home/tester"
	tests := []struct {
		name   string
		shell  string
		goos   string
		zdot   string
		exists map[string]bool
		want   string
	}{
		{"zsh default", ShellZsh, "linux", "", nil, filepath.Join(home, ".zshrc")},
		{"zsh zdotdir", ShellZsh, "darwin", "/config/zsh", nil, "/config/zsh/.zshrc"},
		{"mac bash default", ShellBash, "darwin", "", nil, filepath.Join(home, ".bash_profile")},
		{"mac bash existing profile", ShellBash, "darwin", "", map[string]bool{filepath.Join(home, ".profile"): true}, filepath.Join(home, ".profile")},
		{"linux bash default", ShellBash, "linux", "", nil, filepath.Join(home, ".bashrc")},
		{"linux bash existing login", ShellBash, "linux", "", map[string]bool{filepath.Join(home, ".bash_profile"): true}, filepath.Join(home, ".bash_profile")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Inspect(test.shell, Options{
				Home: home, ZDOTDIR: test.zdot, GOOS: test.goos,
				Exists: func(path string) bool { return test.exists[path] },
				ReadBounded: func(path string, _ int) ([]byte, error) {
					if path != test.want {
						t.Fatalf("read path = %q, want %q", path, test.want)
					}
					return nil, os.ErrNotExist
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if plan.StartupFile != test.want || plan.Activation != `eval "$(intent-sh init `+test.shell+`)"` {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestInspectDetectsOnlyRelevantUnsupportedBindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		shell   string
		content string
		want    []Conflict
	}{
		{ShellZsh, "# bindkey '^[g' ignored\nbindkey '^[g' custom-rewrite\nbindkey '^M' custom-enter\nbindkey '^[x' other\nbindkey '^[u' intent-sh-undo\n", []Conflict{{Backend: ConflictBackendNative, Key: "Alt+G"}, {Backend: ConflictBackendNative, Key: "Enter (CR)"}}},
		{ShellBash, `bind '"\\eg": custom'` + "\n" + `bind -x '"\\C-j":other'` + "\n" + `bind -x '"\\eu":__intent_sh_undo'` + "\n", []Conflict{{Backend: ConflictBackendNative, Key: "Alt+G"}, {Backend: ConflictBackendNative, Key: "Enter (LF)"}}},
	}
	for _, test := range tests {
		plan, err := Inspect(test.shell, Options{
			Home: "/home/tester", GOOS: "linux",
			Exists:      func(string) bool { return true },
			ReadBounded: func(string, int) ([]byte, error) { return []byte(test.content), nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plan.Conflicts, test.want) {
			t.Fatalf("%s conflicts = %#v, want %#v", test.shell, plan.Conflicts, test.want)
		}
	}
}

func TestInspectWithBindingsUsesEffectiveCustomKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		shell   string
		content string
	}{
		{ShellZsh, "bindkey '^X' custom-rewrite\nbindkey $'\\x1b\\x27' custom-undo\nbindkey '^[g' unrelated-default\n"},
		{ShellBash, `bind -x '"\C-x":custom-rewrite'` + "\n" + `bind -x '"\x1b\x27":custom-undo'` + "\n" + `bind -x '"\eg":unrelated-default'` + "\n"},
	}
	for _, test := range tests {
		t.Run(test.shell, func(t *testing.T) {
			plan, err := InspectWithBindings(test.shell, Options{
				Home: "/home/tester", GOOS: "linux",
				Exists:      func(string) bool { return true },
				ReadBounded: func(string, int) ([]byte, error) { return []byte(test.content), nil },
			}, "CTRL+X", "alt+'")
			if err != nil {
				t.Fatal(err)
			}
			want := []Conflict{{Backend: ConflictBackendNative, Key: "Ctrl+X"}, {Backend: ConflictBackendNative, Key: "Alt+'"}}
			if !reflect.DeepEqual(plan.Conflicts, want) {
				t.Fatalf("conflicts = %#v, want %#v", plan.Conflicts, want)
			}
			if plan.RewriteKey != "ctrl+x" || plan.UndoKey != "alt+'" || !strings.Contains(strings.Join(plan.Bindings, "\n"), "Ctrl+X") {
				t.Fatalf("effective plan = %#v", plan)
			}
		})
	}
}

func TestInspectWithBindingsRejectsDuplicateReservedAndAdversarialValues(t *testing.T) {
	t.Parallel()
	options := Options{
		Home: "/home/tester", GOOS: "linux",
		ReadBounded: func(string, int) ([]byte, error) {
			t.Fatal("invalid binding reached startup-file inspection")
			return nil, nil
		},
	}
	for _, test := range []struct {
		name    string
		rewrite string
		undo    string
		want    string
	}{
		{name: "duplicate", rewrite: "ALT+G", undo: "alt+g", want: "distinct"},
		{name: "reserved", rewrite: "ctrl+c", undo: "alt+u", want: "rewrite_key"},
		{name: "injection", rewrite: "alt+g;touch /tmp/nope", undo: "alt+u", want: "rewrite_key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := InspectWithBindings(ShellZsh, options, test.rewrite, test.undo)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	plan, err := InspectWithBindings(ShellZsh, Options{
		Home: "/home/tester", GOOS: "linux", Exists: func(string) bool { return false },
		ReadBounded: func(string, int) ([]byte, error) { return nil, os.ErrNotExist },
	}, "alt+;", "alt+'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(plan.Bindings, "\n"), "Alt+;") || !strings.Contains(strings.Join(plan.Bindings, "\n"), "Alt+'") {
		t.Fatalf("adversarial punctuation was not rendered as bounded display text: %#v", plan.Bindings)
	}
}

func TestInspectBashReportsBleshContractConflictsAndLoadOrder(t *testing.T) {
	t.Parallel()
	content := `eval "$(intent-sh init bash)"
source "$HOME/.local/share/blesh/ble.sh"
ble-bind -x M-g custom-rewrite
ble-bind -x 'M-u' custom-undo
ble/function#advice around ble/widget/default/accept-line 'custom-advice'
`
	plan, err := Inspect(ShellBash, Options{
		Home:        "/home/tester",
		GOOS:        "darwin",
		Exists:      func(string) bool { return true },
		ReadBounded: func(string, int) ([]byte, error) { return []byte(content), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	wantConflicts := []Conflict{
		{Backend: ConflictBackendBlesh, Key: "Alt+G"},
		{Backend: ConflictBackendBlesh, Key: "Alt+U"},
		{Backend: ConflictBackendBlesh, Key: "accept-line"},
	}
	if !reflect.DeepEqual(plan.Conflicts, wantConflicts) {
		t.Fatalf("conflicts = %#v, want %#v", plan.Conflicts, wantConflicts)
	}
	if !plan.BleshLoadOrderConflict {
		t.Fatal("intent-sh-before-ble.sh load order was not detected")
	}
	if plan.BleshVersion == "" || plan.BleshCommit != BleshCommit || plan.BleshInstallURL != BleshInstallURL {
		t.Fatalf("ble.sh guidance = %#v", plan)
	}
}

func TestInspectBashAcceptsBleshBeforeIntentActivation(t *testing.T) {
	t.Parallel()
	content := `source "$HOME/.local/share/blesh/ble.sh"
ble-attach
eval "$(intent-sh init bash)"
`
	plan, err := Inspect(ShellBash, Options{
		Home:        "/home/tester",
		GOOS:        "linux",
		Exists:      func(string) bool { return true },
		ReadBounded: func(string, int) ([]byte, error) { return []byte(content), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.BleshLoadOrderConflict {
		t.Fatal("correct ble.sh load order was reported as conflicting")
	}
}

func TestInspectNeverWritesOrExecutesStartupFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	marker := filepath.Join(dir, "executed")
	content := []byte("touch " + marker + "\nbindkey '^[g' custom\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Inspect(ShellZsh, Options{Home: dir, GOOS: "darwin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v", plan.Conflicts)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, content) {
		t.Fatal("startup file was modified")
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup content was executed: %v", err)
	}
}

func TestInspectRejectsUnsupportedShellAndOversizeFile(t *testing.T) {
	t.Parallel()
	if _, err := Inspect("fish", Options{Home: "/tmp"}); err == nil {
		t.Fatal("unsupported shell unexpectedly accepted")
	}
	_, err := Inspect(ShellZsh, Options{
		Home:        "/tmp",
		ReadBounded: func(string, int) ([]byte, error) { return nil, errors.New("too large") },
	})
	if err == nil {
		t.Fatal("inspection error unexpectedly accepted")
	}
}

func TestReadBoundedRegularFileRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readBoundedRegularFile(path, maxStartupBytes); err == nil {
		t.Fatal("FIFO startup path unexpectedly accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("FIFO startup path blocked inspection")
	}
}
