package contextinfo

import (
	"errors"
	"reflect"
	"testing"
)

func TestBuilderUsesOnlyAllowlistedSignals(t *testing.T) {
	env := map[string]string{
		"SSH_CONNECTION": "SECRET_REMOTE_ADDRESS",
		"LC_ALL":         "en_US.UTF-8",
		"DATABASE_URL":   "SECRET_DATABASE",
		"API_TOKEN":      "SECRET_TOKEN",
	}
	looked := []string{}
	b := Builder{
		GOOS:   "darwin",
		GOARCH: "arm64",
		Getenv: func(key string) string { return env[key] },
		LookPath: func(name string) (string, error) {
			looked = append(looked, name)
			if name == "git" || name == "rg" {
				return "/secret/path/" + name, nil
			}
			return "", errors.New("missing")
		},
	}
	got := b.Build("zsh", "5.9", "/Users/alice/project")
	if !got.Remote || got.Locale != "en_US.UTF-8" {
		t.Fatalf("signals = %#v", got)
	}
	if !reflect.DeepEqual(got.AvailableTools, []string{"git", "rg"}) {
		t.Fatalf("tools = %#v", got.AvailableTools)
	}
	if !reflect.DeepEqual(looked, ToolAllowlist) {
		t.Fatalf("looked paths = %#v", looked)
	}
	if got.CWD != "/Users/alice/project" || got.OS != "darwin" || got.Arch != "arm64" {
		t.Fatalf("context = %#v", got)
	}
}

func TestLocalePriorityAndSanitization(t *testing.T) {
	env := map[string]string{"LC_ALL": "zh_CN.UTF-8\nSECRET", "LANG": "ignored"}
	b := Builder{GOOS: "linux", GOARCH: "amd64", Getenv: func(k string) string { return env[k] }, LookPath: func(string) (string, error) { return "", errors.New("missing") }}
	got := b.Build("bash\n", "5.2\r", "/tmp")
	if got.Locale != "zh_CN.UTF-8SECRET" || got.Shell != "bash" || got.ShellVersion != "5.2" {
		t.Fatalf("sanitized = %#v", got)
	}
}
