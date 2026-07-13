package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

func TestPathFor(t *testing.T) {
	got, err := PathFor("/home/alice", "")
	if err != nil || got != "/home/alice/.config/intent-sh/config.toml" {
		t.Fatalf("PathFor() = %q, %v", got, err)
	}
	got, err = PathFor("/home/alice", "/tmp/xdg")
	if err != nil || got != "/tmp/xdg/intent-sh/config.toml" {
		t.Fatalf("PathFor(XDG) = %q, %v", got, err)
	}
}

func TestLoadAtMissingUsesDefaultsWithoutCreating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.toml")
	got, err := LoadAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Defaults()) {
		t.Fatalf("LoadAt() = %#v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing config was created: %v", err)
	}
}

func TestLoadAtPartialAndStrict(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "partial.toml")
	if err := os.WriteFile(partial, []byte("provider = \"codex\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAt(partial)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != ProviderCodex || got.TimeoutSeconds != 30 || len(got.Priority) != 2 {
		t.Fatalf("partial config did not retain defaults: %#v", got)
	}
	unknown := filepath.Join(dir, "unknown.toml")
	if err := os.WriteFile(unknown, []byte("secret_token = \"nope\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAt(unknown); err == nil || !strings.Contains(err.Error(), "unknown field(s): secret_token") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestValidateTable(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"provider", func(c *Config) { c.Provider = "other" }},
		{"empty priority", func(c *Config) { c.Priority = nil }},
		{"unknown priority", func(c *Config) { c.Priority = []string{"other"} }},
		{"duplicate priority", func(c *Config) { c.Priority = []string{"codex", "codex"} }},
		{"small timeout", func(c *Config) { c.TimeoutSeconds = 0 }},
		{"large timeout", func(c *Config) { c.TimeoutSeconds = 121 }},
		{"multiline model", func(c *Config) { c.Model = "one\ntwo" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSetAtWritesAtomicallyWithPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intent-sh", "config.toml")
	cfg, err := SetAt(path, "provider", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != ProviderCodex {
		t.Fatalf("provider = %q", cfg.Provider)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o", got)
	}
	loaded, err := LoadAt(path)
	if err != nil || loaded.Provider != ProviderCodex {
		t.Fatalf("LoadAt() = %#v, %v", loaded, err)
	}
	if _, err := SetAt(path, "token", "SECRET"); err == nil {
		t.Fatal("credential-like unknown key was accepted")
	}
}

func TestSetAtPriorityAndTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := SetAt(path, "priority", "codex, claude"); err != nil {
		t.Fatal(err)
	}
	cfg, err := SetAt(path, "timeout_seconds", "45")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Priority, []string{"codex", "claude"}) || cfg.TimeoutSeconds != 45 {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestInvalidTOMLDoesNotEchoCredentialValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	secret := "SECRET_CONFIG_CREDENTIAL_SENTINEL"
	if err := os.WriteFile(path, []byte("provider = \"auto\"\nmodel = \""+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAt(path)
	if err == nil {
		t.Fatal("invalid TOML unexpectedly loaded")
	}
	for _, output := range []string{err.Error(), apperr.Message(err)} {
		if strings.Contains(output, secret) {
			t.Fatalf("configuration value leaked in %q", output)
		}
	}
	if !strings.Contains(apperr.Message(err), "line") || !strings.Contains(apperr.Message(err), "column") {
		t.Fatalf("safe correction location omitted: %q", apperr.Message(err))
	}
}

func TestLoadAtRejectsOversizeAndSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	oversize := filepath.Join(dir, "oversize.toml")
	if err := os.WriteFile(oversize, []byte(strings.Repeat("#", maxConfigBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAt(oversize); apperr.KindOf(err) != apperr.KindConfiguration {
		t.Fatalf("oversize kind = %q, want configuration; err=%v", apperr.KindOf(err), err)
	}

	fifo := filepath.Join(dir, "config.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := LoadAt(fifo); apperr.KindOf(err) != apperr.KindConfiguration {
		t.Fatalf("FIFO kind = %q, want configuration; err=%v", apperr.KindOf(err), err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("FIFO configuration path blocked loading")
	}
}
