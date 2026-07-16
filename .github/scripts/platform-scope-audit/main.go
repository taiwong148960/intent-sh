package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	maxFileBytes = 4 << 20
	maxFindings  = 100
)

var (
	// These identifiers describe required macOS and POSIX implementation
	// details; they are deliberately outside the prohibited-reference pattern.
	allowedPlatformIdentifiers = []string{"darwin", "unix", "posix", "golang.org/x/sys/unix"}
	prohibitedReferencePattern = regexp.MustCompile(`(?i)(?:\b(?:linux|ubuntu|debian|fedora|centos|alpine|windows|wsl|freebsd|openbsd|netbsd|elf|glibc|musl)\b|\bapt(?:-get)?\b|\b(?:dnf|yum|pacman)\b|(?:cross|multi)[ -]?platform)`)
	auditedRoots               = []string{".codex", ".github", "cmd", "docs", "internal", "openspec", "schemas", "shell"}
	auditedRootFiles           = []string{".gitignore", "Makefile", "README.md", "go.mod"}
)

type finding struct {
	path string
	line int
	text string
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail("locate repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fail("platform scope audit must run from the repository root")
	}

	paths, err := auditPaths(root)
	if err != nil {
		fail("inventory platform scope: %v", err)
	}
	findings, err := scanPaths(root, paths)
	if err != nil {
		fail("scan platform scope: %v", err)
	}
	if len(findings) != 0 {
		for _, item := range findings {
			fmt.Fprintf(os.Stderr, "%s:%d: %s\n", item.path, item.line, item.text)
		}
		fail("platform scope audit found %d prohibited first-party reference(s)", len(findings))
	}
	fmt.Printf("platform scope audit passed; allowed implementation identifiers: %s\n", strings.Join(allowedPlatformIdentifiers, ", "))
}

func auditPaths(root string) ([]string, error) {
	paths := append([]string(nil), auditedRootFiles...)
	for _, relativeRoot := range auditedRoots {
		absoluteRoot := filepath.Join(root, relativeRoot)
		err := filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if excludedPath(relative, entry.IsDir()) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if !entry.IsDir() && textFile(relative) {
				paths = append(paths, relative)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func excludedPath(relative string, directory bool) bool {
	if relative == ".git" || strings.HasPrefix(relative, ".git/") {
		return true
	}
	if relative == "openspec/changes/archive" || strings.HasPrefix(relative, "openspec/changes/archive/") {
		return true
	}
	// The active migration record must describe the boundary it removes. Once
	// archived, it is covered by the permanent historical-record exclusion.
	if relative == "openspec/changes/establish-macos-only-baseline" || strings.HasPrefix(relative, "openspec/changes/establish-macos-only-baseline/") {
		return true
	}
	if directory && (filepath.Base(relative) == "node_modules" || relative == ".github/ci-tools/bin") {
		return true
	}
	if filepath.Base(relative) == "package-lock.json" || relative == "go.sum" {
		return true
	}
	return relative == ".github/scripts/platform-scope-audit/main.go"
}

func textFile(relative string) bool {
	base := filepath.Base(relative)
	if base == "Makefile" || strings.HasPrefix(base, ".gitignore") {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".bash", ".env", ".go", ".json", ".md", ".mod", ".sh", ".toml", ".txt", ".yaml", ".yml", ".zsh":
		return true
	default:
		return false
	}
}

func scanPaths(root string, paths []string) ([]finding, error) {
	var findings []finding
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
			return nil, fmt.Errorf("%s is not a bounded regular text file", relative)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil, fmt.Errorf("%s contains binary data", relative)
		}
		for index, line := range strings.Split(string(data), "\n") {
			if !prohibitedReferencePattern.MatchString(line) {
				continue
			}
			findings = append(findings, finding{path: relative, line: index + 1, text: strings.TrimSpace(line)})
			if len(findings) == maxFindings {
				return findings, nil
			}
		}
	}
	return findings, nil
}

func fail(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", arguments...)
	os.Exit(1)
}
