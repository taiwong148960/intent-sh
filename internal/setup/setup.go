// Package setup inspects shell startup files and produces reversible guidance.
package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/keychord"
	"github.com/taiwong148960/intent-sh/internal/protocol"
)

const maxStartupBytes = 64 * 1024

const (
	ShellBash = "bash"
	ShellZsh  = "zsh"

	ConflictBackendNative = "native"
	ConflictBackendBlesh  = "blesh"

	BleshCommit     = "d69e4d549a1881a37300fe6b4a05478bd9157dfc"
	BleshInstallURL = "https://github.com/akinomyoga/ble.sh"
)

// Conflict identifies a default key whose existing startup-file binding may
// be replaced when the adapter loads. It deliberately excludes the source line.
type Conflict struct {
	Backend string
	Key     string
}

// Plan is read-only setup guidance for one supported shell.
type Plan struct {
	Shell                  string
	StartupFile            string
	Activation             string
	Bindings               []string
	RewriteKey             string
	UndoKey                string
	Conflicts              []Conflict
	BleshVersion           string
	BleshCommit            string
	BleshInstallURL        string
	BleshLoadOrderConflict bool
}

// Options makes startup-file discovery deterministic in tests.
type Options struct {
	Home        string
	ZDOTDIR     string
	GOOS        string
	Exists      func(string) bool
	ReadBounded func(string, int) ([]byte, error)
}

// InspectDefault inspects the current user's likely startup file without
// changing it or executing any of its contents.
func InspectDefault(shell string) (Plan, error) {
	return InspectDefaultWithBindings(shell, "alt+g", "alt+u")
}

// InspectDefaultWithBindings inspects the startup file for the effective
// validated native ZLE/Readline bindings.
func InspectDefaultWithBindings(shell, rewriteValue, undoValue string) (Plan, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Plan{}, apperr.Wrap(apperr.KindConfiguration, "prepare setup", "could not determine the home directory", err)
	}
	return InspectWithBindings(shell, Options{
		Home:        home,
		ZDOTDIR:     os.Getenv("ZDOTDIR"),
		GOOS:        runtime.GOOS,
		Exists:      regularFileExists,
		ReadBounded: readBoundedRegularFile,
	}, rewriteValue, undoValue)
}

// Inspect selects a likely startup file and detects static keybinding conflicts.
func Inspect(shell string, options Options) (Plan, error) {
	return InspectWithBindings(shell, options, "alt+g", "alt+u")
}

// InspectWithBindings selects a likely startup file and detects static
// conflicts for the effective native bindings.
func InspectWithBindings(shell string, options Options, rewriteValue, undoValue string) (Plan, error) {
	if shell != ShellBash && shell != ShellZsh {
		return Plan{}, apperr.New(apperr.KindInvalidInput, "prepare setup", "supported shells are zsh and bash")
	}
	if strings.TrimSpace(options.Home) == "" {
		return Plan{}, apperr.New(apperr.KindConfiguration, "prepare setup", "HOME is unset")
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.Exists == nil {
		options.Exists = regularFileExists
	}
	if options.ReadBounded == nil {
		options.ReadBounded = readBoundedRegularFile
	}
	rewrite, err := keychord.Parse(rewriteValue)
	if err != nil {
		return Plan{}, apperr.New(apperr.KindConfiguration, "prepare setup", "rewrite_key is invalid: "+err.Error())
	}
	undo, err := keychord.Parse(undoValue)
	if err != nil {
		return Plan{}, apperr.New(apperr.KindConfiguration, "prepare setup", "undo_key is invalid: "+err.Error())
	}
	if rewrite == undo {
		return Plan{}, apperr.New(apperr.KindConfiguration, "prepare setup", "rewrite_key and undo_key must be distinct")
	}

	plan := Plan{
		Shell:      shell,
		Activation: `eval "$(intent-sh init ` + shell + `)"`,
		RewriteKey: rewrite.Canonical(),
		UndoKey:    undo.Canonical(),
		Bindings: []string{
			rewrite.Display() + ": rewrite the current buffer; press again to regenerate",
			undo.Display() + ": restore the original buffer when it is still unchanged",
			"Enter: normal acceptance, with a two-Enter guard for dangerous generated commands",
			"Ctrl+C: cancel an in-progress rewrite",
		},
	}
	if shell == ShellBash {
		plan.BleshVersion = protocol.BleshVersion
		plan.BleshCommit = BleshCommit
		plan.BleshInstallURL = BleshInstallURL
	}
	plan.StartupFile = startupFile(shell, options)

	data, err := options.ReadBounded(plan.StartupFile, maxStartupBytes)
	if errors.Is(err, os.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return Plan{}, apperr.Wrap(apperr.KindConfiguration, "inspect shell setup", "could not safely inspect the shell startup file", err)
	}
	content := string(data)
	plan.Conflicts = detectConflicts(shell, content, rewrite, undo)
	if shell == ShellBash {
		plan.BleshLoadOrderConflict = detectBleshLoadOrderConflict(content)
	}
	return plan, nil
}

func startupFile(shell string, options Options) string {
	if shell == ShellZsh {
		base := options.Home
		if strings.TrimSpace(options.ZDOTDIR) != "" {
			base = options.ZDOTDIR
		}
		return filepath.Join(base, ".zshrc")
	}

	candidates := []string{".bashrc", ".bash_profile", ".bash_login", ".profile"}
	defaultName := ".bashrc"
	if options.GOOS == "darwin" {
		candidates = []string{".bash_profile", ".bash_login", ".profile", ".bashrc"}
		defaultName = ".bash_profile"
	}
	for _, name := range candidates {
		path := filepath.Join(options.Home, name)
		if options.Exists(path) {
			return path
		}
	}
	return filepath.Join(options.Home, defaultName)
}

func detectConflicts(shell, content string, rewrite, undo keychord.Chord) []Conflict {
	nativeKeys := []struct {
		name     string
		patterns []string
	}{
		{name: rewrite.Display(), patterns: bindingPatterns(rewrite)},
		{name: undo.Display(), patterns: bindingPatterns(undo)},
		{name: "Enter (CR)", patterns: []string{"^M", `\C-m`, `\C-M`, `\\C-m`, `\\C-M`, `\x0d`, `\\x0d`}},
		{name: "Enter (LF)", patterns: []string{"^J", `\C-j`, `\C-J`, `\\C-j`, `\\C-J`, `\x0a`, `\\x0a`}},
	}
	bleshKeys := []struct {
		name string
		key  string
	}{
		{name: "Alt+G", key: "M-g"},
		{name: "Alt+U", key: "M-u"},
	}
	found := make(map[string]bool, len(nativeKeys)+len(bleshKeys)+1)
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "intent-sh-") || strings.Contains(line, "__intent_sh_") {
			continue
		}

		backend := ConflictBackendNative
		if shell == ShellZsh {
			if !strings.Contains(line, "bindkey") {
				continue
			}
		} else if containsBleshBind(line) {
			backend = ConflictBackendBlesh
		} else if !containsBashBind(line) {
			continue
		}

		if backend == ConflictBackendBlesh {
			for _, key := range bleshKeys {
				if containsShellWord(line, key.key) {
					found[backend+"\x00"+key.name] = true
				}
			}
			if strings.Contains(line, "ble/function#advice") && strings.Contains(line, "accept-line") {
				found[backend+"\x00accept-line"] = true
			}
			continue
		}

		for _, key := range nativeKeys {
			for _, pattern := range key.patterns {
				if strings.Contains(line, pattern) {
					found[backend+"\x00"+key.name] = true
					break
				}
			}
		}
	}
	conflicts := make([]Conflict, 0, len(found))
	for _, key := range nativeKeys {
		if found[ConflictBackendNative+"\x00"+key.name] {
			conflicts = append(conflicts, Conflict{Backend: ConflictBackendNative, Key: key.name})
		}
	}
	for _, key := range bleshKeys {
		if found[ConflictBackendBlesh+"\x00"+key.name] {
			conflicts = append(conflicts, Conflict{Backend: ConflictBackendBlesh, Key: key.name})
		}
	}
	if found[ConflictBackendBlesh+"\x00accept-line"] {
		conflicts = append(conflicts, Conflict{Backend: ConflictBackendBlesh, Key: "accept-line"})
	}
	return conflicts
}

func bindingPatterns(chord keychord.Chord) []string {
	key := chord.Key()
	hex := chord.ZLEBinding()
	patterns := []string{hex, strings.ReplaceAll(hex, `\`, `\\`)}
	if chord.Modifier() == keychord.ModifierAlt {
		patterns = append(patterns,
			"^["+string(key),
			`\e`+string(key), `\\e`+string(key),
			`\M-`+string(key), `\\M-`+string(key),
		)
	} else {
		upper := strings.ToUpper(string(key))
		patterns = append(patterns,
			"^"+upper,
			fmt.Sprintf(`\C-%c`, key), fmt.Sprintf(`\C-%s`, upper),
			fmt.Sprintf(`\\C-%c`, key), fmt.Sprintf(`\\C-%s`, upper),
		)
	}
	return patterns
}

func containsBashBind(line string) bool {
	return strings.HasPrefix(line, "bind ") || strings.HasPrefix(line, "builtin bind ") || strings.HasPrefix(line, "command bind ")
}

func containsBleshBind(line string) bool {
	return strings.HasPrefix(line, "ble-bind ") || strings.HasPrefix(line, "command ble-bind ") ||
		strings.Contains(line, "ble/function#advice") && strings.Contains(line, "accept-line")
}

func containsShellWord(line, want string) bool {
	for _, field := range strings.Fields(line) {
		if strings.Trim(field, `"'`) == want {
			return true
		}
	}
	return false
}

func detectBleshLoadOrderConflict(content string) bool {
	intentLine := -1
	bleshLine := -1
	for index, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if intentLine < 0 && strings.Contains(line, "intent-sh init bash") {
			intentLine = index
		}
		if bleshLine < 0 && (strings.Contains(line, "ble.sh") || strings.Contains(line, "ble-attach")) {
			bleshLine = index
		}
	}
	return intentLine >= 0 && bleshLine >= 0 && intentLine < bleshLine
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func readBoundedRegularFile(path string, limit int) ([]byte, error) {
	// Nonblocking open prevents a startup path replaced with a FIFO from
	// hanging setup. It has no effect when the opened object is regular.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("startup path is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("startup file exceeds inspection limit")
	}
	return data, nil
}
