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

func TestBashThreeAndKeyConflictsFailClosed(t *testing.T) {
	t.Parallel()
	deps := healthyDependencies()
	deps.ShellPath = "/bin/bash"
	deps.ShellVersion = func(context.Context, string) (string, error) { return "GNU bash, version 3.2.57", nil }
	deps.InspectSetup = func(string) (setupguide.Plan, error) {
		return setupguide.Plan{Conflicts: []setupguide.Conflict{{Key: "Alt+G"}, {Key: "Enter (CR)"}}}, nil
	}
	report := (Runner{Dependencies: deps}).Run(context.Background())
	checks := checksByID(report)
	if report.Ready || checks["shell.compatibility"].Status != StatusFail || checks["shell.default_keys"].Status != StatusFail {
		t.Fatalf("report = %#v", report)
	}
	output := render(report)
	for _, want := range []string{"below the 4.0 minimum", "stock Zsh", "Alt+G, Enter (CR)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output omitted %q:\n%s", want, output)
		}
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
		InspectSetup:  func(string) (setupguide.Plan, error) { return setupguide.Plan{}, nil },
		LookPath:      func(string) (string, error) { return "/bin/provider", nil },
		ShellVersion:  func(context.Context, string) (string, error) { return "zsh 5.9", nil },
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
