// Package config loads and atomically updates secret-free user configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pelletier/go-toml/v2"
	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/keychord"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

const (
	ProviderAuto   = "auto"
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
)

const maxConfigBytes = 64 * 1024

// Config contains only secret-free provider and local binding preferences.
type Config struct {
	Provider       string   `toml:"provider" json:"provider"`
	Priority       []string `toml:"priority" json:"priority"`
	TimeoutSeconds int      `toml:"timeout_seconds" json:"timeoutSeconds"`
	Model          string   `toml:"model" json:"model"`
	RewriteKey     string   `toml:"rewrite_key" json:"rewriteKey"`
	UndoKey        string   `toml:"undo_key" json:"undoKey"`
}

func Defaults() Config {
	return Config{
		Provider:       ProviderAuto,
		Priority:       []string{ProviderClaude, ProviderCodex},
		TimeoutSeconds: 30,
		Model:          "",
		RewriteKey:     "alt+g",
		UndoKey:        "alt+u",
	}
}

// Path returns the XDG configuration path for the current process.
func Path() (string, error) {
	return PathFor(os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"))
}

// PathFor is separated for deterministic tests.
func PathFor(home, xdg string) (string, error) {
	base := strings.TrimSpace(xdg)
	if base == "" {
		if strings.TrimSpace(home) == "" {
			return "", apperr.New(apperr.KindConfiguration, "resolve config path", "HOME and XDG_CONFIG_HOME are both unset")
		}
		base = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(base) {
		return "", apperr.New(apperr.KindConfiguration, "resolve config path", "XDG_CONFIG_HOME must be an absolute path")
	}
	return filepath.Join(base, "intent-sh", "config.toml"), nil
}

func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}
	cfg, err := LoadAt(path)
	return cfg, path, err
}

// LoadAt returns defaults without creating a file when path does not exist.
func LoadAt(path string) (Config, error) {
	cfg := Defaults()
	data, err := readBoundedConfigFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, apperr.Wrap(apperr.KindConfiguration, "load config", "could not read intent-sh configuration", err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, apperr.Wrap(apperr.KindConfiguration, "load config", decodeMessage(err), err)
	}
	cfg, err = cfg.normalized()
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func readBoundedConfigFile(path string) ([]byte, error) {
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
		return nil, errors.New("configuration path is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigBytes {
		return nil, errors.New("configuration file exceeds the size limit")
	}
	return data, nil
}

func (c Config) Validate() error {
	_, err := c.normalized()
	return err
}

func (c Config) normalized() (Config, error) {
	if !validProviderMode(c.Provider) {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "provider must be one of auto, claude, or codex")
	}
	if len(c.Priority) == 0 {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "priority must contain at least one provider")
	}
	seen := make(map[string]bool, len(c.Priority))
	for _, provider := range c.Priority {
		if provider != ProviderClaude && provider != ProviderCodex {
			return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "priority contains an unknown provider; use only claude or codex")
		}
		if seen[provider] {
			return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "priority contains a duplicate provider")
		}
		seen[provider] = true
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 120 {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "timeout_seconds must be between 1 and 120")
	}
	if len(c.Model) > 200 || strings.ContainsAny(c.Model, "\x00\r\n") {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "model must be at most 200 characters on one line")
	}
	rewrite, err := keychord.Parse(c.RewriteKey)
	if err != nil {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "rewrite_key is invalid: "+err.Error())
	}
	undo, err := keychord.Parse(c.UndoKey)
	if err != nil {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "undo_key is invalid: "+err.Error())
	}
	if rewrite == undo {
		return Config{}, apperr.New(apperr.KindConfiguration, "validate config", "rewrite_key and undo_key must be distinct")
	}
	c.RewriteKey = rewrite.Canonical()
	c.UndoKey = undo.Canonical()
	return c, nil
}

func validProviderMode(value string) bool {
	return value == ProviderAuto || value == ProviderClaude || value == ProviderCodex
}

// SetAt changes one supported key and atomically replaces the file.
func SetAt(path, key, value string) (Config, error) {
	cfg, err := LoadAt(path)
	if err != nil {
		return Config{}, err
	}
	switch key {
	case "provider":
		cfg.Provider = value
	case "priority":
		parts := strings.Split(value, ",")
		cfg.Priority = cfg.Priority[:0]
		for _, part := range parts {
			cfg.Priority = append(cfg.Priority, strings.TrimSpace(part))
		}
	case "timeout_seconds":
		n, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return Config{}, apperr.New(apperr.KindConfiguration, "set config", "timeout_seconds must be an integer")
		}
		cfg.TimeoutSeconds = n
	case "model":
		cfg.Model = value
	case "rewrite_key":
		cfg.RewriteKey = value
	case "undo_key":
		cfg.UndoKey = value
	default:
		return Config{}, apperr.New(apperr.KindConfiguration, "set config", "unknown configuration key; use provider, priority, timeout_seconds, model, rewrite_key, or undo_key")
	}
	cfg, err = cfg.normalized()
	if err != nil {
		return Config{}, err
	}
	if err := WriteAt(path, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func WriteAt(path string, cfg Config) error {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not encode intent-sh configuration", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not create the configuration directory", err)
	}
	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not create a temporary configuration file", err)
	}
	tempName := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not protect the temporary configuration file", err)
	}
	if _, err := temp.Write(data); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not write the temporary configuration file", err)
	}
	if err := temp.Sync(); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not sync the temporary configuration file", err)
	}
	if err := temp.Close(); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not close the temporary configuration file", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return apperr.Wrap(apperr.KindConfiguration, "write config", "could not replace the configuration file", err)
	}
	committed = true
	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return nil, err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return nil, apperr.Wrap(apperr.KindConfiguration, "show config", "could not encode intent-sh configuration", err)
	}
	return data, nil
}

func bounded(value string, max int) string {
	return textsafe.Terminal(value, max)
}

func decodeMessage(err error) string {
	var missing *toml.StrictMissingError
	if errors.As(err, &missing) {
		keys := make([]string, 0, len(missing.Errors))
		for _, item := range missing.Errors {
			key := strings.Join(item.Key(), ".")
			if key != "" {
				keys = append(keys, bounded(key, 80))
			}
		}
		if len(keys) > 0 {
			return "configuration contains unknown field(s): " + strings.Join(keys, ", ")
		}
		return "configuration contains an unknown field"
	}
	var decodeErr *toml.DecodeError
	if errors.As(err, &decodeErr) {
		line, column := decodeErr.Position()
		return fmt.Sprintf("configuration TOML is invalid at line %d, column %d", line, column)
	}
	return "configuration is invalid"
}
