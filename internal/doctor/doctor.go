// Package doctor performs read-only local compatibility and readiness checks.
package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/keyprobe"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	setupguide "github.com/taiwong148960/intent-sh/internal/setup"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
	shellassets "github.com/taiwong148960/intent-sh/shell"
)

// Status is a stable, terminal-safe doctor check outcome.
type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusWarn Status = "WARN"
	StatusSkip Status = "SKIP"
)

// Check is one stable diagnostic result.
type Check struct {
	Status   Status
	ID       string
	Detail   string
	Guidance string
}

// Report contains deterministic checks and the overall CLI outcome.
type Report struct {
	Checks      []Check
	Ready       bool
	FailureKind apperr.Kind
}

// AdapterStatus is the bounded compatibility state exported by a loaded shell
// adapter. Values are treated as untrusted capability claims and are never
// copied into doctor output.
type AdapterStatus struct {
	Present       bool
	Invalid       bool
	Protocol      string
	Backend       string
	EditorVersion string
	Ready         string
	Failure       string
	Conflicts     string
}

// Dependencies are explicit so doctor can be tested without inspecting the
// developer's machine or invoking real provider CLIs.
type Dependencies struct {
	GOOS          string
	GOARCH        string
	ShellPath     string
	LoadConfig    func() (config.Config, string, error)
	CheckProtocol func() error
	AdapterStatus func() AdapterStatus
	InspectSetup  func(string) (setupguide.Plan, error)
	// InspectSetupBindings is the production seam for effective configured
	// bindings. InspectSetup remains for focused legacy tests.
	InspectSetupBindings func(string, string, string) (setupguide.Plan, error)
	KeyProbe             func(context.Context, string, string) keyprobe.Result
	LookPath             func(string) (string, error)
	ShellVersion         func(context.Context, string) (string, error)
	Providers            map[string]provider.Provider
}

// Runner executes a configured doctor inspection.
type Runner struct {
	Dependencies Dependencies
}

// NewDefault creates the production read-only diagnostic runner.
func NewDefault() Runner {
	return Runner{Dependencies: Dependencies{
		GOOS:                 runtime.GOOS,
		GOARCH:               runtime.GOARCH,
		ShellPath:            os.Getenv("SHELL"),
		LoadConfig:           config.Load,
		CheckProtocol:        defaultProtocolCheck,
		AdapterStatus:        inspectAdapterStatus,
		InspectSetupBindings: setupguide.InspectDefaultWithBindings,
		KeyProbe: func(ctx context.Context, rewriteKey, undoKey string) keyprobe.Result {
			return (keyprobe.Probe{}).Run(ctx, rewriteKey, undoKey)
		},
		LookPath:     exec.LookPath,
		ShellVersion: inspectShellVersion,
		Providers: map[string]provider.Provider{
			provider.NameClaude: provider.Claude{},
			provider.NameCodex:  provider.Codex{},
		},
	}}
}

// RunKeys performs ordinary non-interactive readiness checks and then the
// explicit bounded controlling-terminal key probe.
func (runner Runner) RunKeys(ctx context.Context) Report {
	report := runner.Run(ctx)
	deps := withDefaults(runner.Dependencies)
	cfg, _, configErr := deps.LoadConfig()
	result := deps.KeyProbe(ctx, cfg.RewriteKey, cfg.UndoKey)
	for _, check := range result.Checks {
		status := StatusFail
		switch check.Status {
		case keyprobe.StatusPass:
			status = StatusPass
		case keyprobe.StatusSkip:
			status = StatusSkip
		}
		report.add(status, check.ID, check.Detail, check.Guidance)
	}
	if configErr != nil || !result.Ready {
		report.Ready = false
		setFailureKind(&report, apperr.KindConfiguration)
	}
	return report
}

// Run performs all checks and never includes raw subprocess errors in a Check.
func (runner Runner) Run(ctx context.Context) Report {
	deps := withDefaults(runner.Dependencies)
	report := Report{Ready: true}
	coreReady := true

	if deps.GOOS == "darwin" || deps.GOOS == "linux" {
		report.add(StatusPass, "platform.os", "supported operating system", "")
	} else {
		report.add(StatusFail, "platform.os", "unsupported operating system", "Use macOS or Linux.")
		coreReady = false
		report.FailureKind = apperr.KindConfiguration
	}
	if deps.GOARCH == "amd64" || deps.GOARCH == "arm64" {
		report.add(StatusPass, "platform.arch", "supported architecture", "")
	} else {
		report.add(StatusFail, "platform.arch", "unsupported architecture", "Use an amd64 or arm64 build.")
		coreReady = false
		setFailureKind(&report, apperr.KindConfiguration)
	}

	cfg, _, configErr := deps.LoadConfig()
	if configErr != nil {
		report.add(StatusFail, "config.valid", apperr.Message(configErr), "Correct the reported field with `intent-sh config set` or edit the secret-free TOML file.")
		coreReady = false
		setFailureKind(&report, apperr.KindConfiguration)
	} else {
		report.add(StatusPass, "config.valid", "configuration is valid and contains no credential fields", "")
	}

	if err := deps.CheckProtocol(); err != nil {
		report.add(StatusFail, "adapter.protocol", "embedded adapters do not match the binary protocol", "Rebuild the binary and reload the shell adapter.")
		coreReady = false
		setFailureKind(&report, apperr.KindProtocol)
	} else {
		report.add(StatusPass, "adapter.protocol", "embedded Bash and Zsh adapters match protocol "+protocol.AdapterVersion, "")
	}

	shellName, shellMajor, shellMinor, shellReady := inspectShell(ctx, deps, &report)
	if !shellReady {
		coreReady = false
		setFailureKind(&report, apperr.KindConfiguration)
	}
	if shellName == "" {
		report.add(StatusSkip, "shell.default_keys", "key conflicts were not inspected", "Select Zsh or Bash, then run doctor again.")
	} else {
		var plan setupguide.Plan
		var err error
		if deps.InspectSetupBindings != nil {
			plan, err = deps.InspectSetupBindings(shellName, cfg.RewriteKey, cfg.UndoKey)
		} else {
			plan, err = deps.InspectSetup(shellName)
		}
		if err != nil {
			report.add(StatusFail, "shell.default_keys", "startup-file keybindings could not be safely inspected", "Inspect the startup file manually before activation.")
			coreReady = false
			setFailureKind(&report, apperr.KindConfiguration)
		} else if len(conflictsForBackend(plan.Conflicts, setupguide.ConflictBackendNative)) > 0 {
			keys := conflictKeys(conflictsForBackend(plan.Conflicts, setupguide.ConflictBackendNative))
			report.add(StatusFail, "shell.default_keys", "custom bindings conflict with: "+strings.Join(keys, ", "), "Review or remove those custom bindings before activation.")
			coreReady = false
			setFailureKind(&report, apperr.KindConfiguration)
		} else {
			report.add(StatusPass, "shell.default_keys", "no static conflicts found for "+plan.RewriteKey+", "+plan.UndoKey+", or Enter", "")
		}
	}

	if !inspectEditorBackend(deps.AdapterStatus(), shellName, shellMajor, shellMinor, shellReady, &report) {
		coreReady = false
		setFailureKind(&report, apperr.KindConfiguration)
	}

	configured := configuredProviders(cfg, configErr)
	health := probeProviders(ctx, deps, configured)
	providerReady := false
	for _, item := range health {
		providerReady = providerReady || item.ready
	}
	for _, name := range []string{provider.NameClaude, provider.NameCodex} {
		item, ok := health[name]
		if !ok {
			addUnconfiguredProvider(&report, name)
			continue
		}
		optionalFailure := providerReady && !item.ready
		addProviderChecks(&report, item, optionalFailure)
	}
	if providerReady {
		report.add(StatusPass, "provider.ready", "at least one configured official provider is compatible and logged in", "")
	} else {
		report.add(StatusFail, "provider.ready", "no configured provider is ready", "Install and use the official login flow for Claude Code or Codex CLI; intent-sh never asks for an API key.")
		setFailureKind(&report, apperr.KindProviderUnavailable)
	}

	report.Ready = coreReady && providerReady
	if report.Ready {
		report.FailureKind = ""
	}
	return report
}

// Render writes bounded, one-line checks suitable for terminals and scripts.
func Render(w io.Writer, report Report) {
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%s %s - %s", check.Status, check.ID, terminalText(check.Detail, 240))
		if check.Guidance != "" {
			fmt.Fprintf(w, "; action: %s", terminalText(check.Guidance, 240))
		}
		fmt.Fprintln(w)
	}
	if report.Ready {
		fmt.Fprintln(w, "READY intent-sh can serve rewrites on this machine")
	} else {
		fmt.Fprintln(w, "NOT_READY resolve the failed checks above")
	}
}

func withDefaults(deps Dependencies) Dependencies {
	defaults := NewDefault().Dependencies
	if deps.GOOS == "" {
		deps.GOOS = defaults.GOOS
	}
	if deps.GOARCH == "" {
		deps.GOARCH = defaults.GOARCH
	}
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.CheckProtocol == nil {
		deps.CheckProtocol = defaults.CheckProtocol
	}
	if deps.AdapterStatus == nil {
		deps.AdapterStatus = defaults.AdapterStatus
	}
	if deps.InspectSetup == nil && deps.InspectSetupBindings == nil {
		deps.InspectSetupBindings = defaults.InspectSetupBindings
	}
	if deps.KeyProbe == nil {
		deps.KeyProbe = defaults.KeyProbe
	}
	if deps.LookPath == nil {
		deps.LookPath = defaults.LookPath
	}
	if deps.ShellVersion == nil {
		deps.ShellVersion = defaults.ShellVersion
	}
	if deps.Providers == nil {
		deps.Providers = defaults.Providers
	}
	return deps
}

func inspectShell(ctx context.Context, deps Dependencies, report *Report) (string, int, int, bool) {
	shellName := strings.TrimPrefix(filepath.Base(strings.TrimSpace(deps.ShellPath)), "-")
	if shellName != "bash" && shellName != "zsh" {
		report.add(StatusFail, "shell.compatibility", "SHELL is not a supported Zsh or Bash executable", "Use Zsh or Bash 4.0 or newer.")
		return "", 0, 0, false
	}
	version, err := deps.ShellVersion(ctx, deps.ShellPath)
	if err != nil {
		report.add(StatusFail, "shell.compatibility", "shell version could not be verified without startup files", "Verify the shell executable and try again.")
		return shellName, 0, 0, false
	}
	major, minor, ok := parseVersion(version)
	if !ok {
		report.add(StatusFail, "shell.compatibility", "shell reported an unrecognized version", "Use Zsh or Bash 4.0 or newer.")
		return shellName, 0, 0, false
	}
	if shellName == "bash" && major < 4 {
		report.add(StatusFail, "shell.compatibility", fmt.Sprintf("Bash %d.%d is below the 4.0 minimum", major, minor), "Use Zsh or install Bash 4.0 or newer; doctor will not modify your system shell.")
		return shellName, major, minor, false
	}
	report.add(StatusPass, "shell.compatibility", fmt.Sprintf("%s %d.%d is supported", strings.Title(shellName), major, minor), "")
	return shellName, major, minor, true
}

func inspectEditorBackend(status AdapterStatus, shellName string, shellMajor, shellMinor int, shellReady bool, report *Report) bool {
	if !shellReady || shellName == "" {
		report.add(StatusSkip, "shell.editor_backend", "active editor backend was not checked", "Resolve shell compatibility first.")
		report.add(StatusSkip, "shell.backend_keys", "backend key conflicts were not checked", "Resolve shell compatibility first.")
		return false
	}

	if !status.Present {
		report.add(StatusFail, "shell.editor_backend", "no initialized adapter status was inherited by doctor", "Evaluate `intent-sh init "+shellName+"` in this shell, then run doctor again.")
		report.add(StatusSkip, "shell.backend_keys", "runtime backend key conflicts were not reported", "Initialize the shell adapter first.")
		return false
	}

	if !validAdapterStatus(status) {
		report.add(StatusFail, "shell.editor_backend", "adapter status markers were invalid or unbounded", "Re-evaluate the embedded adapter and run doctor again.")
		report.add(StatusFail, "shell.backend_keys", "backend key status was invalid", "Reinitialize the adapter without conflicting custom bindings.")
		return false
	}

	protocolOK := status.Protocol == protocol.AdapterVersion
	backendOK := coherentBackend(shellName, status.Backend)
	versionOK := coherentEditorVersion(shellMajor, shellMinor, status)
	ready := status.Ready == "1" && status.Failure == ""
	editorReady := protocolOK && backendOK && versionOK && ready
	if editorReady {
		detail := "active editor backend is " + editorBackendName(status.Backend)
		report.add(StatusPass, "shell.editor_backend", detail, "")
	} else {
		detail, guidance := adapterFailureMessage(status, protocolOK, backendOK, versionOK)
		report.add(StatusFail, "shell.editor_backend", detail, guidance)
	}

	if editorReady {
		report.add(StatusPass, "shell.backend_keys", "active "+editorBackendName(status.Backend)+" backend reported no runtime key conflict", "")
	} else {
		report.add(StatusSkip, "shell.backend_keys", "runtime backend key conflicts were not verified", "Reinitialize the shell adapter.")
	}
	return editorReady
}

func conflictsForBackend(conflicts []setupguide.Conflict, backend string) []setupguide.Conflict {
	result := make([]setupguide.Conflict, 0, len(conflicts))
	for _, conflict := range conflicts {
		actual := conflict.Backend
		if actual == "" {
			actual = setupguide.ConflictBackendNative
		}
		if actual == backend {
			result = append(result, conflict)
		}
	}
	return result
}

func conflictKeys(conflicts []setupguide.Conflict) []string {
	keys := make([]string, 0, len(conflicts))
	seen := make(map[string]bool, len(conflicts))
	for _, conflict := range conflicts {
		key := safeConflictName(conflict.Key)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func safeConflictName(value string) string {
	switch value {
	case "Alt+G", "Alt+U", "Enter (CR)", "Enter (LF)":
		return value
	default:
		return "unknown key"
	}
}

func coherentBackend(shellName, backend string) bool {
	if shellName == setupguide.ShellZsh {
		return backend == protocol.EditorBackendZLE
	}
	return backend == protocol.EditorBackendReadline
}

func coherentEditorVersion(shellMajor, shellMinor int, status AdapterStatus) bool {
	major, minor, ok := parseVersion(status.EditorVersion)
	return ok && major == shellMajor && minor == shellMinor
}

func editorBackendName(backend string) string {
	switch backend {
	case protocol.EditorBackendReadline:
		return "native Readline"
	case protocol.EditorBackendZLE:
		return "ZLE"
	default:
		return "unknown"
	}
}

func adapterFailureMessage(status AdapterStatus, protocolOK, backendOK, versionOK bool) (string, string) {
	if !protocolOK {
		return "loaded adapter protocol does not match binary protocol " + protocol.AdapterVersion, "Re-evaluate the adapter emitted by this binary."
	}
	switch status.Failure {
	case "unsupported_bash":
		return "Bash is below the supported minimum", "Use Bash 4.0 or newer or Zsh."
	case "initializing":
		return "adapter initialization did not complete", "Re-evaluate the embedded adapter."
	case "missing_backend":
		return "native editor state is unavailable", "Invoke intent-sh from an active ZLE or native Readline editing buffer."
	}
	if !backendOK {
		return "reported editor backend is incompatible with this shell version", "Use ZLE for Zsh or native Readline for Bash 4.0 or newer."
	}
	if !versionOK {
		return "reported editor version is incompatible with the selected backend", "Reload the supported editor and reinitialize intent-sh."
	}
	return "adapter did not report a ready editor backend", "Reinitialize the adapter and resolve its bounded failure checks."
}

func validAdapterStatus(status AdapterStatus) bool {
	if status.Invalid || !markerValuesBounded(status) {
		return false
	}
	if status.Protocol == "" || status.Ready != "0" && status.Ready != "1" {
		return false
	}
	switch status.Backend {
	case "none", protocol.EditorBackendZLE, protocol.EditorBackendReadline:
	default:
		return false
	}
	switch status.Failure {
	case "", "initializing", "unsupported_bash", "missing_backend":
	default:
		return false
	}
	switch status.Conflicts {
	case "":
	default:
		return false
	}
	if status.Ready == "1" && (status.Failure != "" || status.Backend == "none") {
		return false
	}
	return true
}

func markerValuesBounded(status AdapterStatus) bool {
	for _, value := range []string{status.Protocol, status.Backend, status.EditorVersion, status.Ready, status.Failure, status.Conflicts} {
		if len(value) > 96 {
			return false
		}
		for _, char := range value {
			if char < 0x20 || char == 0x7f {
				return false
			}
		}
	}
	return true
}

func inspectAdapterStatus() AdapterStatus {
	status := AdapterStatus{}
	markers := []struct {
		name   string
		target *string
	}{
		{"INTENT_SH_ADAPTER_PROTOCOL", &status.Protocol},
		{"INTENT_SH_ADAPTER_BACKEND", &status.Backend},
		{"INTENT_SH_ADAPTER_EDITOR_VERSION", &status.EditorVersion},
		{"INTENT_SH_ADAPTER_READY", &status.Ready},
		{"INTENT_SH_ADAPTER_FAILURE", &status.Failure},
		{"INTENT_SH_ADAPTER_CONFLICTS", &status.Conflicts},
	}
	for _, marker := range markers {
		value, ok := os.LookupEnv(marker.name)
		if !ok {
			continue
		}
		status.Present = true
		if len(value) > 96 {
			status.Invalid = true
			continue
		}
		*marker.target = value
	}
	if !markerValuesBounded(status) {
		status.Invalid = true
	}
	return status
}

type providerHealth struct {
	name       string
	found      bool
	versionOK  bool
	featuresOK bool
	loginOK    bool
	ready      bool
}

func configuredProviders(cfg config.Config, configErr error) []string {
	if configErr != nil {
		return nil
	}
	if cfg.Provider == config.ProviderAuto {
		return append([]string(nil), cfg.Priority...)
	}
	return []string{cfg.Provider}
}

func probeProviders(ctx context.Context, deps Dependencies, names []string) map[string]providerHealth {
	unique := make(map[string]bool, len(names))
	for _, name := range names {
		if name == provider.NameClaude || name == provider.NameCodex {
			unique[name] = true
		}
	}
	type result struct {
		name   string
		health providerHealth
	}
	results := make(chan result, len(unique))
	for name := range unique {
		name := name
		go func() { results <- result{name: name, health: probeProvider(ctx, deps, name)} }()
	}
	health := make(map[string]providerHealth, len(unique))
	for range unique {
		item := <-results
		health[item.name] = item.health
	}
	return health
}

func probeProvider(ctx context.Context, deps Dependencies, name string) providerHealth {
	health := providerHealth{name: name}
	if _, err := deps.LookPath(name); err != nil {
		return health
	}
	health.found = true
	adapter := deps.Providers[name]
	if adapter == nil {
		return health
	}
	result, err := adapter.Probe(ctx)
	if result.Version != "" {
		health.versionOK = true
	}
	if err == nil {
		health.versionOK = true
		health.featuresOK = true
		health.loginOK = true
		health.ready = true
		return health
	}
	stage, ok := provider.ProbeStageOf(err)
	if !ok {
		return health
	}
	switch stage {
	case provider.ProbeStageFeatures:
		health.versionOK = true
	case provider.ProbeStageLogin:
		health.versionOK = true
		health.featuresOK = true
	}
	return health
}

func addProviderChecks(report *Report, health providerHealth, optionalFailure bool) {
	failureStatus := StatusFail
	if optionalFailure {
		failureStatus = StatusWarn
	}
	prefix := "provider." + health.name + "."
	if !health.found {
		report.add(failureStatus, prefix+"executable", "official CLI executable was not found on PATH", installGuidance(health.name))
		report.add(StatusSkip, prefix+"version", "version was not checked", "Install the official CLI first.")
		report.add(StatusSkip, prefix+"features", "required isolation and structured-output flags were not checked", "Install the official CLI first.")
		report.add(StatusSkip, prefix+"login", "official login readiness was not checked", loginGuidance(health.name))
		return
	}
	report.add(StatusPass, prefix+"executable", "official CLI executable was found on PATH", "")
	if !health.versionOK {
		report.add(failureStatus, prefix+"version", "CLI version could not be verified", "Update or reinstall the official CLI.")
		report.add(StatusSkip, prefix+"features", "required flags were not checked", "Verify a compatible CLI version first.")
		report.add(StatusSkip, prefix+"login", "official login readiness was not checked", loginGuidance(health.name))
		return
	}
	report.add(StatusPass, prefix+"version", "CLI reported a compatible version", "")
	if !health.featuresOK {
		report.add(failureStatus, prefix+"features", "required isolation or structured-output flags are unavailable", "Update the official CLI before enabling rewrites.")
		report.add(StatusSkip, prefix+"login", "official login readiness was not checked", loginGuidance(health.name))
		return
	}
	report.add(StatusPass, prefix+"features", "required isolation and structured-output flags are available", "")
	if !health.loginOK {
		report.add(failureStatus, prefix+"login", "official CLI login is not ready", loginGuidance(health.name))
		return
	}
	report.add(StatusPass, prefix+"login", "official CLI login is ready", "")
}

func addUnconfiguredProvider(report *Report, name string) {
	for _, suffix := range []string{"executable", "version", "features", "login"} {
		report.add(StatusSkip, "provider."+name+"."+suffix, "provider is not selected by the effective configuration", "")
	}
}

func installGuidance(name string) string {
	if name == provider.NameClaude {
		return "Install the official Claude Code CLI, then run `claude` and use `/login`."
	}
	return "Install the official Codex CLI, then run `codex login`."
}

func loginGuidance(name string) string {
	if name == provider.NameClaude {
		return "Run `claude`, then use the official `/login` flow."
	}
	return "Run the official `codex login` flow."
}

func defaultProtocolCheck() error {
	for _, name := range []string{"bash", "zsh"} {
		if _, err := shellassets.Script(name, protocol.AdapterVersion); err != nil {
			return err
		}
	}
	return nil
}

var versionPattern = regexp.MustCompile(`([0-9]+)\.([0-9]+)`)

func parseVersion(value string) (int, int, bool) {
	parts := versionPattern.FindStringSubmatch(value)
	if len(parts) != 3 {
		return 0, 0, false
	}
	major, errMajor := strconv.Atoi(parts[1])
	minor, errMinor := strconv.Atoi(parts[2])
	return major, minor, errMajor == nil && errMinor == nil
}

func inspectShellVersion(ctx context.Context, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("shell path is empty")
	}
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, path, "--version")
	stdout := &limitedBuffer{limit: 4096}
	stderr := &limitedBuffer{limit: 4096}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	if stdout.truncated || stderr.truncated {
		return "", errors.New("shell version output exceeded limit")
	}
	value := strings.TrimSpace(stdout.String())
	if value == "" {
		value = strings.TrimSpace(stderr.String())
	}
	return value, nil
}

type limitedBuffer struct {
	limit     int
	data      bytes.Buffer
	truncated bool
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - buffer.data.Len()
	if remaining > 0 {
		keep := len(data)
		if keep > remaining {
			keep = remaining
		}
		_, _ = buffer.data.Write(data[:keep])
	}
	if len(data) > remaining {
		buffer.truncated = true
	}
	return len(data), nil
}

func (buffer *limitedBuffer) String() string { return buffer.data.String() }

func (report *Report) add(status Status, id, detail, guidance string) {
	report.Checks = append(report.Checks, Check{Status: status, ID: id, Detail: detail, Guidance: guidance})
}

func setFailureKind(report *Report, kind apperr.Kind) {
	if report.FailureKind == "" || report.FailureKind == apperr.KindProviderUnavailable {
		report.FailureKind = kind
	}
}

func terminalText(value string, limit int) string {
	return textsafe.Terminal(strings.TrimSpace(value), limit)
}

// StableIDs returns the expected diagnostic identifiers for contract tests.
func StableIDs() []string {
	ids := []string{
		"adapter.protocol", "config.valid", "platform.arch", "platform.os",
		"provider.ready", "shell.backend_keys",
		"shell.compatibility", "shell.default_keys", "shell.editor_backend",
	}
	for _, name := range []string{provider.NameClaude, provider.NameCodex} {
		for _, suffix := range []string{"executable", "features", "login", "version"} {
			ids = append(ids, "provider."+name+"."+suffix)
		}
	}
	sort.Strings(ids)
	return ids
}
