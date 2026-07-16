package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/taiwong148960/intent-sh/internal/provider"
)

var versionPattern = regexp.MustCompile(`^[A-Za-z0-9._+() -]{1,120}$`)

type result struct {
	Provider string `json:"provider"`
	Version  string `json:"version"`
	Status   string `json:"status"`
	Stage    string `json:"stage"`
}

func main() {
	codexPath := flag.String("codex", "", "pinned Codex CLI executable")
	claudePath := flag.String("claude", "", "pinned Claude Code executable")
	home := flag.String("home", "", "empty probe home")
	flag.Parse()
	if flag.NArg() != 0 {
		fatal(errors.New("unexpected provider probe arguments"))
	}
	for _, path := range []string{*codexPath, *claudePath} {
		if err := regularExecutable(path); err != nil {
			fatal(err)
		}
	}
	if err := privateDirectory(*home); err != nil {
		fatal(err)
	}
	codexHome := filepath.Join(*home, "codex")
	claudeHome := filepath.Join(*home, "claude")
	temporary := filepath.Join(*home, "tmp")
	for _, directory := range []string{codexHome, claudeHome, temporary} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			fatal(errors.New("create isolated provider probe directory"))
		}
	}
	environment := []string{
		"HOME=" + *home,
		"CODEX_HOME=" + codexHome,
		"CLAUDE_CONFIG_DIR=" + claudeHome,
		"TMPDIR=" + temporary,
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
		"LC_CTYPE=en_US.UTF-8",
		"PATH=" + filepath.Dir(*codexPath) + string(os.PathListSeparator) + filepath.Dir(*claudePath) + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	runner := provider.ProcessRunner{Env: environment}
	probes := []struct {
		name    string
		adapter provider.Provider
	}{
		{name: provider.NameCodex, adapter: provider.Codex{Runner: runner, Program: *codexPath}},
		{name: provider.NameClaude, adapter: provider.Claude{Runner: runner, Program: *claudePath}},
	}
	results := make([]result, 0, len(probes))
	for _, probe := range probes {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		metadata, err := probe.adapter.Probe(ctx)
		cancel()
		if err == nil {
			fatal(fmt.Errorf("%s unexpectedly found authenticated state in its empty home", probe.name))
		}
		stage, ok := provider.ProbeStageOf(err)
		if !ok || stage != provider.ProbeStageLogin {
			fatal(fmt.Errorf("%s failed before the expected login-not-ready stage", probe.name))
		}
		version := strings.TrimSpace(metadata.Version)
		if !versionPattern.MatchString(version) {
			fatal(fmt.Errorf("%s returned unsafe or missing version metadata", probe.name))
		}
		results = append(results, result{Provider: probe.name, Version: version, Status: "pass", Stage: string(stage)})
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(results); err != nil {
		fatal(errors.New("encode bounded provider probe result"))
	}
}

func regularExecutable(path string) error {
	if len(path) == 0 || len(path) > 500 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("provider probe executable path is unsafe")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("provider probe executable must be a regular executable")
	}
	return nil
}

func privateDirectory(path string) error {
	if len(path) == 0 || len(path) > 500 || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == string(filepath.Separator) {
		return errors.New("provider probe home path is unsafe")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("provider probe home must be a private real directory")
	}
	return nil
}

func fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
