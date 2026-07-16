package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/artifactqual"
)

const maxManifestBytes = 64 << 10

type manifest struct {
	Schema  int                   `json:"schema"`
	Builder string                `json:"builder"`
	Flags   []string              `json:"flags"`
	Targets []artifactqual.Report `json:"targets"`
}

func main() {
	if len(os.Args) < 2 {
		fatal(errors.New("usage: artifact-qualify build|inspect -dir PATH"))
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	directory := flags.String("dir", "", "artifact directory")
	if err := flags.Parse(os.Args[2:]); err != nil {
		fatal(err)
	}
	if flags.NArg() != 0 || !safeDirectory(*directory) {
		fatal(errors.New("-dir must be one bounded absolute clean path"))
	}
	switch os.Args[1] {
	case "build":
		fatal(buildAll(*directory))
	case "inspect":
		fatal(inspectAll(*directory))
	default:
		fatal(errors.New("usage: artifact-qualify build|inspect -dir PATH"))
	}
}

func buildAll(directory string) error {
	if err := prepareEmptyDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Dir(directory), ".intent-sh-artifact-build.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)

	reports := make([]artifactqual.Report, 0, len(artifactqual.SupportedTargets))
	for _, target := range artifactqual.SupportedTargets {
		first := filepath.Join(temporary, target.Filename()+".first")
		second := filepath.Join(temporary, target.Filename()+".second")
		if err := buildTarget(first, target); err != nil {
			return err
		}
		if err := buildTarget(second, target); err != nil {
			return err
		}
		firstData, err := os.ReadFile(first)
		if err != nil {
			return err
		}
		secondData, err := os.ReadFile(second)
		if err != nil {
			return err
		}
		if !bytes.Equal(firstData, secondData) {
			return fmt.Errorf("target %s/%s was not reproducible", target.GOOS, target.GOARCH)
		}
		destination := filepath.Join(directory, target.Filename())
		if err := os.Rename(first, destination); err != nil {
			return err
		}
		report, err := artifactqual.Inspect(destination, target)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", target.Filename(), err)
		}
		reports = append(reports, report)
	}
	return writeMetadata(directory, reports)
}

func inspectAll(directory string) error {
	data, err := readBoundedRegular(filepath.Join(directory, "manifest.json"), maxManifestBytes)
	if err != nil {
		return err
	}
	var expected manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		return fmt.Errorf("decode artifact manifest: %w", err)
	}
	if expected.Schema != 1 || expected.Builder != "go build" || len(expected.Targets) != len(artifactqual.SupportedTargets) {
		return errors.New("artifact manifest header or target count is invalid")
	}
	if strings.Join(expected.Flags, "\x00") != strings.Join(buildFlags(), "\x00") {
		return errors.New("artifact manifest build flags changed")
	}
	checksumData, err := readBoundedRegular(filepath.Join(directory, "checksums.txt"), maxManifestBytes)
	if err != nil {
		return err
	}
	for index, target := range artifactqual.SupportedTargets {
		report, err := artifactqual.Inspect(filepath.Join(directory, target.Filename()), target)
		if err != nil {
			return err
		}
		if report != expected.Targets[index] {
			return fmt.Errorf("artifact metadata changed for %s", target.Filename())
		}
		line := report.SHA256 + "  " + report.Filename + "\n"
		if bytes.Count(checksumData, []byte(line)) != 1 {
			return fmt.Errorf("checksum metadata missing or duplicated for %s", target.Filename())
		}
	}
	if bytes.Count(checksumData, []byte("\n")) != len(artifactqual.SupportedTargets) {
		return errors.New("checksum metadata contains unexpected entries")
	}
	encoded, _ := json.Marshal(struct {
		Status  string `json:"status"`
		Targets int    `json:"targets"`
	}{Status: "pass", Targets: len(expected.Targets)})
	fmt.Println(string(encoded))
	return nil
}

func buildTarget(destination string, target artifactqual.Target) error {
	arguments := append([]string{"build"}, buildFlags()...)
	arguments = append(arguments, "-o", destination, "./cmd/intent-sh")
	command := exec.Command("go", arguments...)
	command.Env = replaceEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0", "GOOS": target.GOOS, "GOARCH": target.GOARCH,
		"GOTOOLCHAIN": "local", "GOFLAGS": "-mod=readonly",
	})
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("build %s/%s: %w: %s", target.GOOS, target.GOARCH, err, bounded(output.String(), 1024))
	}
	return nil
}

func buildFlags() []string {
	return []string{"-trimpath", "-buildvcs=false", "-ldflags=-buildid="}
}

func writeMetadata(directory string, reports []artifactqual.Report) error {
	value := manifest{Schema: 1, Builder: "go build", Flags: buildFlags(), Targets: reports}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxManifestBytes {
		return errors.New("artifact manifest exceeded its bound")
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), data, 0o600); err != nil {
		return err
	}
	var checksums strings.Builder
	for _, report := range reports {
		fmt.Fprintf(&checksums, "%s  %s\n", report.SHA256, report.Filename)
	}
	return os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(checksums.String()), 0o600)
}

func prepareEmptyDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(directory, 0o700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("artifact output must be a real directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("artifact output directory must be empty")
	}
	return nil
}

func readBoundedRegular(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, errors.New("metadata must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit+1))
}

func safeDirectory(value string) bool {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || value == string(filepath.Separator) || len(value) > 500 {
		return false
	}
	for _, component := range strings.Split(filepath.ToSlash(value), "/") {
		if component == "." || component == ".." || strings.ContainsAny(component, "\x00\r\n") {
			return false
		}
	}
	return true
}

func replaceEnvironment(source []string, replacements map[string]string) []string {
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(source)+len(replacements))
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := replacements[key]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func bounded(value string, maximum int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, value)
	if len(value) > maximum {
		value = value[:maximum]
	}
	return value
}

func fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, bounded(err.Error(), 1024))
	os.Exit(1)
}
