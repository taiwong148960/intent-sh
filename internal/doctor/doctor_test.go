package doctor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/keyprobe"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	setupguide "github.com/taiwong148960/intent-sh/internal/setup"
)

type fakeProvider struct {
	name   string
	result provider.ProbeResult
	err    error
}

func (fake fakeProvider) Name() string { return fake.name }
func (fake fakeProvider) Generate(context.Context, provider.Request) (protocol.ProviderResult, error) {
	return protocol.ProviderResult{}, errors.New("not used")
}
func (fake fakeProvider) Probe(context.Context) (provider.ProbeResult, error) {
	return fake.result, fake.err
}

func TestReadyWhenOneAutoProviderIsUsable(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.LookPath = func(name string) (string, error) {
		if name == provider.NameCodex {
			return "/bin/codex", nil
		}
		return "", os.ErrNotExist
	}
	deps.Providers = map[string]provider.Provider{
		provider.NameCodex: fakeProvider{name: provider.NameCodex, result: provider.ProbeResult{Provider: provider.NameCodex, Version: "SECRET_VERSION_OUTPUT"}},
	}
	report := (Runner{Dependencies: deps}).Run(context.Background())
	if !report.Ready || report.FailureKind != "" {
		t.Fatalf("report = %#v", report)
	}
	wantIDs := StableIDs()
	gotIDs := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		gotIDs = append(gotIDs, check.ID)
	}
	sort.Strings(gotIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ids = %#v, want %#v", gotIDs, wantIDs)
	}
	output := render(report)
	for _, want := range []string{
		"WARN provider.claude.executable",
		"PASS provider.codex.features",
		"PASS provider.codex.login",
		"PASS provider.ready",
		"READY intent-sh",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output omitted %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "SECRET_VERSION_OUTPUT") {
		t.Fatalf("raw version output leaked:\n%s", output)
	}
}

func TestRunKeysAppendsInteractiveChecksAndUsesConfiguredChords(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.LoadConfig = func() (config.Config, string, error) {
		cfg := config.Defaults()
		cfg.RewriteKey = "ctrl+x"
		cfg.UndoKey = "alt+'"
		return cfg, "", nil
	}
	var gotRewrite, gotUndo string
	deps.KeyProbe = func(_ context.Context, rewriteKey, undoKey string) keyprobe.Result {
		gotRewrite, gotUndo = rewriteKey, undoKey
		checks := make([]keyprobe.Check, 0, 6)
		for _, id := range []string{keyprobe.CheckTTY, keyprobe.CheckRewrite, keyprobe.CheckUndo, keyprobe.CheckEnter, keyprobe.CheckCancel, keyprobe.CheckRestore} {
			checks = append(checks, keyprobe.Check{Status: keyprobe.StatusPass, ID: id, Detail: "passed"})
		}
		return keyprobe.Result{Checks: checks, Ready: true}
	}
	report := (Runner{Dependencies: deps}).RunKeys(context.Background())
	if !report.Ready || gotRewrite != "ctrl+x" || gotUndo != "alt+'" {
		t.Fatalf("report=%#v configured keys=%q/%q", report, gotRewrite, gotUndo)
	}
	checks := checksByID(report)
	for _, id := range []string{keyprobe.CheckTTY, keyprobe.CheckRewrite, keyprobe.CheckUndo, keyprobe.CheckEnter, keyprobe.CheckCancel, keyprobe.CheckRestore} {
		if checks[id].Status != StatusPass {
			t.Fatalf("key check %s = %#v", id, checks[id])
		}
	}
}

func TestRunKeysFailsActionablyWithoutControllingTerminal(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.KeyProbe = func(context.Context, string, string) keyprobe.Result {
		checks := []keyprobe.Check{{Status: keyprobe.StatusFail, ID: keyprobe.CheckTTY, Detail: "controlling terminal is unavailable", Guidance: "run directly from a terminal"}}
		for _, id := range []string{keyprobe.CheckRewrite, keyprobe.CheckUndo, keyprobe.CheckEnter, keyprobe.CheckCancel, keyprobe.CheckRestore} {
			checks = append(checks, keyprobe.Check{Status: keyprobe.StatusSkip, ID: id, Detail: "not checked"})
		}
		return keyprobe.Result{Checks: checks}
	}
	report := (Runner{Dependencies: deps}).RunKeys(context.Background())
	checks := checksByID(report)
	if report.Ready || report.FailureKind != apperr.KindConfiguration || checks[keyprobe.CheckTTY].Status != StatusFail {
		t.Fatalf("report = %#v", report)
	}
	if checks[keyprobe.CheckRewrite].Status != StatusSkip || !strings.Contains(render(report), "run directly from a terminal") {
		t.Fatalf("actionable skipped checks missing: %#v", report)
	}
}

func TestNoProviderInstalledFailsWithOfficialLoginGuidance(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	report := (Runner{Dependencies: deps}).Run(context.Background())
	if report.Ready || report.FailureKind != apperr.KindProviderUnavailable {
		t.Fatalf("report = %#v", report)
	}
	output := render(report)
	for _, want := range []string{"FAIL provider.claude.executable", "FAIL provider.codex.executable", "codex login", "/login", "NOT_READY"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output omitted %q:\n%s", want, output)
		}
	}
	if strings.Contains(strings.ToLower(output), "api key") && !strings.Contains(output, "never asks for an API key") {
		t.Fatalf("doctor unexpectedly requested an API key:\n%s", output)
	}
}

func TestProviderProbeStageChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		stage       provider.ProbeStage
		wantPass    []string
		wantFail    string
		wantSkipped string
	}{
		{"version", provider.ProbeStageVersion, nil, "provider.codex.version", "provider.codex.features"},
		{"features", provider.ProbeStageFeatures, []string{"provider.codex.version"}, "provider.codex.features", "provider.codex.login"},
		{"login", provider.ProbeStageLogin, []string{"provider.codex.version", "provider.codex.features"}, "provider.codex.login", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := healthyDependencies()
			deps.LoadConfig = func() (config.Config, string, error) {
				cfg := config.Defaults()
				cfg.Provider = config.ProviderCodex
				return cfg, "", nil
			}
			deps.LookPath = func(string) (string, error) { return "/bin/codex", nil }
			result := provider.ProbeResult{Provider: provider.NameCodex}
			if test.stage != provider.ProbeStageVersion {
				result.Version = "0.1"
			}
			probeCause := apperr.Wrap(apperr.KindProviderUnavailable, "probe", "safe probe failure", errors.New("SECRET_PROVIDER_STDERR"))
			deps.Providers = map[string]provider.Provider{
				provider.NameCodex: fakeProvider{name: provider.NameCodex, result: result, err: &provider.ProbeError{Stage: test.stage, Err: probeCause}},
			}
			report := (Runner{Dependencies: deps}).Run(context.Background())
			if report.Ready {
				t.Fatal("failed explicit provider unexpectedly ready")
			}
			checks := checksByID(report)
			if checks[test.wantFail].Status != StatusFail {
				t.Fatalf("failed check = %#v", checks[test.wantFail])
			}
			for _, id := range test.wantPass {
				if checks[id].Status != StatusPass {
					t.Fatalf("pass check %s = %#v", id, checks[id])
				}
			}
			if test.wantSkipped != "" && checks[test.wantSkipped].Status != StatusSkip {
				t.Fatalf("skip check %s = %#v", test.wantSkipped, checks[test.wantSkipped])
			}
			if strings.Contains(render(report), "SECRET_PROVIDER_STDERR") {
				t.Fatal("provider error cause leaked")
			}
		})
	}
}

func TestUnsupportedBashFailsBeforeEditorDiagnostics(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.ShellPath = "/bin/bash"
	deps.ShellVersion = func(context.Context, string) (string, error) { return "GNU bash, version 3.2.57", nil }
	deps.AdapterStatus = func() AdapterStatus {
		return AdapterStatus{
			Present: true, Protocol: protocol.AdapterVersion, Backend: protocol.EditorBackendReadline,
			EditorVersion: "3.2.57", Ready: "1",
		}
	}
	report := (Runner{Dependencies: deps}).Run(context.Background())
	checks := checksByID(report)
	if report.Ready || checks["shell.compatibility"].Status != StatusFail || checks["shell.editor_backend"].Status != StatusSkip {
		t.Fatalf("report = %#v", report)
	}
	if checks["shell.backend_keys"].Status != StatusSkip {
		t.Fatalf("check shell.backend_keys = %#v", checks["shell.backend_keys"])
	}
	output := render(report)
	for _, want := range []string{"below the 4.0 minimum", "Resolve shell compatibility first"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output omitted %q:\n%s", want, output)
		}
	}
}

func TestSupportedBashWithNativeReadlineIsReady(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.ShellPath = "/bin/bash"
	deps.ShellVersion = func(context.Context, string) (string, error) { return "GNU bash, version 5.2.37", nil }
	deps.AdapterStatus = func() AdapterStatus {
		return AdapterStatus{
			Present: true, Protocol: protocol.AdapterVersion, Backend: protocol.EditorBackendReadline,
			EditorVersion: "5.2.37", Ready: "1",
		}
	}
	report := (Runner{Dependencies: deps}).Run(context.Background())
	checks := checksByID(report)
	if !report.Ready {
		t.Fatalf("report = %#v", report)
	}
	for _, id := range []string{"shell.compatibility", "shell.editor_backend", "shell.backend_keys"} {
		if checks[id].Status != StatusPass {
			t.Fatalf("check %s = %#v", id, checks[id])
		}
	}
}

func TestBashNativeBackendFailuresAreSpecificAndFailClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     AdapterStatus
		checkID    string
		wantDetail string
	}{
		{
			name: "native readline version mismatch",
			status: AdapterStatus{Present: true, Protocol: protocol.AdapterVersion, Backend: protocol.EditorBackendReadline,
				EditorVersion: "4.0.0(1)-release", Ready: "1"},
			checkID: "shell.editor_backend", wantDetail: "editor version is incompatible",
		},
		{
			name: "unsupported backend",
			status: AdapterStatus{Present: true, Protocol: protocol.AdapterVersion, Backend: "alternate",
				EditorVersion: "5.2.37", Ready: "1"},
			checkID: "shell.editor_backend", wantDetail: "invalid or unbounded",
		},
		{
			name: "native editor unavailable",
			status: AdapterStatus{Present: true, Protocol: protocol.AdapterVersion, Backend: "none",
				EditorVersion: "none", Ready: "0", Failure: "missing_backend"},
			checkID: "shell.editor_backend", wantDetail: "native editor state is unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := healthyDependencies()
			deps.ShellPath = "/bin/bash"
			deps.ShellVersion = func(context.Context, string) (string, error) { return "GNU bash, version 5.2.37", nil }
			deps.AdapterStatus = func() AdapterStatus { return test.status }
			report := (Runner{Dependencies: deps}).Run(context.Background())
			check := checksByID(report)[test.checkID]
			if report.Ready || check.Status != StatusFail || !strings.Contains(check.Detail, test.wantDetail) {
				t.Fatalf("check %s = %#v; report ready=%v", test.checkID, check, report.Ready)
			}
		})
	}
}

func TestDoctorNeverPrintsUnboundedAdapterOrBindingValues(t *testing.T) {
	t.Parallel()
	secret := "SECRET_INTENT_GENERATED_BINDING_CREDENTIAL"
	deps := healthyDependencies()
	deps.ShellPath = "/bin/bash"
	deps.ShellVersion = func(context.Context, string) (string, error) { return "GNU bash, version 5.2.37", nil }
	deps.AdapterStatus = func() AdapterStatus {
		return AdapterStatus{
			Present: true, Protocol: protocol.AdapterVersion, Backend: strings.Repeat(secret, 20),
			EditorVersion: secret + "\x1b[31m", Ready: "1", Failure: secret, Conflicts: secret,
		}
	}
	deps.InspectSetup = func(string) (setupguide.Plan, error) {
		return setupguide.Plan{Conflicts: []setupguide.Conflict{
			{Backend: setupguide.ConflictBackendNative, Key: secret},
		}}, nil
	}
	report := (Runner{Dependencies: deps}).Run(context.Background())
	output := render(report)
	if report.Ready || !strings.Contains(output, "invalid or unbounded") {
		t.Fatalf("adversarial report = %#v", report)
	}
	if strings.Contains(output, secret) || strings.Contains(output, "\x1b") {
		t.Fatalf("untrusted adapter or binding value leaked:\n%s", output)
	}
	if !strings.Contains(output, "unknown key") {
		t.Fatalf("bounded unknown-key diagnostic missing:\n%s", output)
	}
}

func TestInspectAdapterStatusAcceptsOnlyBoundedMarkers(t *testing.T) {
	values := map[string]string{
		"INTENT_SH_ADAPTER_PROTOCOL":       protocol.AdapterVersion,
		"INTENT_SH_ADAPTER_BACKEND":        protocol.EditorBackendReadline,
		"INTENT_SH_ADAPTER_EDITOR_VERSION": "5.2.37",
		"INTENT_SH_ADAPTER_READY":          "1",
		"INTENT_SH_ADAPTER_FAILURE":        "",
		"INTENT_SH_ADAPTER_CONFLICTS":      "",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	status := inspectAdapterStatus()
	if !status.Present || status.Invalid || !validAdapterStatus(status) {
		t.Fatalf("valid environment markers rejected: %#v", status)
	}
	t.Setenv("INTENT_SH_ADAPTER_CONFLICTS", strings.Repeat("x", 97))
	status = inspectAdapterStatus()
	if !status.Invalid || validAdapterStatus(status) {
		t.Fatalf("unbounded environment marker accepted: %#v", status)
	}
}

func TestConfigAndRenderingRedactInternalCauseAndControls(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.LoadConfig = func() (config.Config, string, error) {
		return config.Config{}, "", apperr.Wrap(apperr.KindConfiguration, "load", "configuration TOML is invalid at line 2, column 4", errors.New("SECRET_CONFIG_VALUE"))
	}
	deps.ShellVersion = func(context.Context, string) (string, error) { return "zsh 5.9\x1b[31mSECRET_SHELL", nil }
	report := (Runner{Dependencies: deps}).Run(context.Background())
	output := render(report)
	if strings.Contains(output, "SECRET_CONFIG_VALUE") || strings.Contains(output, "SECRET_SHELL") || strings.Contains(output, "\x1b") {
		t.Fatalf("sensitive/control content leaked:\n%s", output)
	}
	if !strings.Contains(output, "configuration TOML is invalid at line 2, column 4") {
		t.Fatalf("safe configuration correction was omitted:\n%s", output)
	}
}

func TestProtocolAndUnsupportedShellFailuresHaveStableIDs(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.CheckProtocol = func() error { return errors.New("SECRET_PROTOCOL_CAUSE") }
	deps.ShellPath = "/usr/bin/fish"
	report := (Runner{Dependencies: deps}).Run(context.Background())
	checks := checksByID(report)
	if checks["adapter.protocol"].Status != StatusFail || checks["shell.compatibility"].Status != StatusFail || checks["shell.default_keys"].Status != StatusSkip {
		t.Fatalf("checks = %#v", checks)
	}
	if strings.Contains(render(report), "SECRET_PROTOCOL_CAUSE") {
		t.Fatal("protocol cause leaked")
	}
}

func healthyDependencies() Dependencies {
	return Dependencies{
		GOOS:      "linux",
		GOARCH:    "amd64",
		ShellPath: "/bin/zsh",
		LoadConfig: func() (config.Config, string, error) {
			return config.Defaults(), "/home/test/.config/intent-sh/config.toml", nil
		},
		CheckProtocol: func() error { return nil },
		AdapterStatus: func() AdapterStatus {
			return AdapterStatus{
				Present: true, Protocol: protocol.AdapterVersion, Backend: protocol.EditorBackendZLE,
				EditorVersion: "5.9", Ready: "1",
			}
		},
		InspectSetup: func(string) (setupguide.Plan, error) { return setupguide.Plan{}, nil },
		LookPath:     func(string) (string, error) { return "/bin/provider", nil },
		ShellVersion: func(context.Context, string) (string, error) { return "zsh 5.9", nil },
		Providers: map[string]provider.Provider{
			provider.NameClaude: fakeProvider{name: provider.NameClaude, result: provider.ProbeResult{Provider: provider.NameClaude, Version: "1.0"}},
			provider.NameCodex:  fakeProvider{name: provider.NameCodex, result: provider.ProbeResult{Provider: provider.NameCodex, Version: "1.0"}},
		},
	}
}

func checksByID(report Report) map[string]Check {
	result := make(map[string]Check, len(report.Checks))
	for _, check := range report.Checks {
		result[check.ID] = check
	}
	return result
}

func render(report Report) string {
	var output bytes.Buffer
	Render(&output, report)
	return output.String()
}
