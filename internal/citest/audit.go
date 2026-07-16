// Package citest audits go test -json output against the repository's CI test
// manifest. It intentionally records test identities and final states only;
// arbitrary test output is never copied into qualification artifacts.
package citest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const (
	ManifestVersion  = 1
	DefaultMaxBytes  = 32 << 20
	DefaultMaxEvents = 500_000
)

var (
	packageNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	testNamePattern    = regexp.MustCompile(`^(?:Test|Fuzz)[A-Za-z0-9_]+$`)
	phaseNamePattern   = regexp.MustCompile(`^(?:Test|Fuzz)[A-Za-z0-9_]+(?:/[A-Za-z0-9_.+:-]{1,160})+$`)
	metadataPattern    = regexp.MustCompile(`^[A-Za-z0-9._+():/-]{1,80}$`)
	matrixPatterns     = map[string]*regexp.Regexp{
		"arch":     regexp.MustCompile(`^(?:amd64|arm64)$`),
		"bash":     regexp.MustCompile(`^[0-9][A-Za-z0-9._+()-]{0,39}$`),
		"fixture":  regexp.MustCompile(`^(?:bash-4\.0|bash-5\.3|zsh-5\.8\.1|zsh-5\.9\.1)$`),
		"go":       regexp.MustCompile(`^go1\.[0-9]{1,3}(?:\.[0-9]{1,3})?[A-Za-z0-9._+-]{0,24}$`),
		"os":       regexp.MustCompile(`^(?:darwin|linux)$`),
		"provider": regexp.MustCompile(`^(?:claude|codex|both)$`),
		"repeat":   regexp.MustCompile(`^(?:2|3|5)$`),
		"seed":     regexp.MustCompile(`^[1-9][0-9]{0,18}$`),
		"target":   regexp.MustCompile(`^external$`),
		"tmux":     regexp.MustCompile(`^[0-9][A-Za-z0-9._+-]{0,39}$`),
		"zsh":      regexp.MustCompile(`^[0-9][A-Za-z0-9._+-]{0,39}$`),
	}
)

type Manifest struct {
	Version int              `json:"version"`
	Suites  map[string]Suite `json:"suites"`
}

type Suite struct {
	Tier          string        `json:"tier"`
	Description   string        `json:"description"`
	Prerequisites []string      `json:"prerequisites"`
	Matrix        []string      `json:"matrix"`
	Expected      []Expectation `json:"expected"`
}

type Expectation struct {
	Package   string `json:"package"`
	Name      string `json:"name"`
	Minimum   int    `json:"minimum,omitempty"`
	AllowSkip bool   `json:"allowSkip,omitempty"`
}

type TestResult struct {
	Package string  `json:"package"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Phase   string  `json:"phase,omitempty"`
	Elapsed float64 `json:"elapsedSeconds,omitempty"`
}

type Counts struct {
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
	Missing    int `json:"missing"`
	Unexpected int `json:"unexpected"`
}

type Summary struct {
	SchemaVersion int               `json:"schemaVersion"`
	Manifest      int               `json:"manifestVersion"`
	Suite         string            `json:"suite"`
	Tier          string            `json:"tier"`
	Matrix        map[string]string `json:"matrix,omitempty"`
	Valid         bool              `json:"valid"`
	Counts        Counts            `json:"counts"`
	Tests         []TestResult      `json:"tests"`
	Problems      []string          `json:"problems,omitempty"`
}

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

// LoadManifest reads and validates a bounded manifest document.
func LoadManifest(r io.Reader, maxBytes int64) (Manifest, error) {
	data, err := readBounded(r, maxBytes)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %d", manifest.Version)
	}
	if len(manifest.Suites) == 0 {
		return errors.New("manifest has no suites")
	}

	for name, suite := range manifest.Suites {
		if !metadataPattern.MatchString(name) {
			return fmt.Errorf("suite name %q is invalid", name)
		}
		if !metadataPattern.MatchString(suite.Tier) {
			return fmt.Errorf("suite %q has invalid tier", name)
		}
		if len(suite.Expected) == 0 {
			return fmt.Errorf("suite %q has no expectations", name)
		}
		seen := make(map[string]bool)
		wildcardPackages := make(map[string]bool)
		exactPackages := make(map[string]bool)
		for _, expectation := range suite.Expected {
			if !packageNamePattern.MatchString(expectation.Package) {
				return fmt.Errorf("suite %q has invalid package name", name)
			}
			if expectation.Name != "*" && !testNamePattern.MatchString(expectation.Name) {
				return fmt.Errorf("suite %q has invalid test name", name)
			}
			if expectation.Minimum < 0 {
				return fmt.Errorf("suite %q has a negative minimum", name)
			}
			key := expectation.Package + "\x00" + expectation.Name
			if seen[key] {
				return fmt.Errorf("suite %q has duplicate expectation", name)
			}
			seen[key] = true
			if expectation.Name == "*" {
				wildcardPackages[expectation.Package] = true
			} else {
				exactPackages[expectation.Package] = true
			}
		}
		for packageName := range wildcardPackages {
			if exactPackages[packageName] {
				return fmt.Errorf("suite %q mixes wildcard and exact expectations for one package", name)
			}
		}
	}
	return nil
}

// Audit checks a bounded stream of go test -json events for one manifest suite.
func Audit(manifest Manifest, suiteName string, r io.Reader, maxBytes int64, maxEvents int) (Summary, error) {
	suite, ok := manifest.Suites[suiteName]
	if !ok {
		return Summary{}, fmt.Errorf("unknown suite %q", suiteName)
	}
	if maxEvents <= 0 {
		maxEvents = DefaultMaxEvents
	}
	data, err := readBounded(r, maxBytes)
	if err != nil {
		return Summary{}, fmt.Errorf("read test events: %w", err)
	}

	observed := make(map[string]TestResult)
	packageFailures := make(map[string]bool)
	nestedSkips := make(map[string]bool)
	nestedFailures := make(map[string]string)
	decoder := json.NewDecoder(bytes.NewReader(data))
	eventCount := 0
	for {
		var event testEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Summary{}, fmt.Errorf("decode test event: %w", err)
		}
		eventCount++
		if eventCount > maxEvents {
			return Summary{}, fmt.Errorf("test event limit exceeded (%d)", maxEvents)
		}
		if event.Test == "" {
			if event.Action == "fail" && packageNamePattern.MatchString(event.Package) {
				packageFailures[event.Package] = true
			}
			continue
		}
		if strings.Contains(event.Test, "/") {
			parent, _, _ := strings.Cut(event.Test, "/")
			if event.Action == "skip" && packageNamePattern.MatchString(event.Package) && testNamePattern.MatchString(parent) {
				nestedSkips[event.Package+"\x00"+parent+"\x00"+event.Test] = true
			}
			if event.Action == "fail" && packageNamePattern.MatchString(event.Package) && testNamePattern.MatchString(parent) && phaseNamePattern.MatchString(event.Test) {
				nestedFailures[event.Package+"\x00"+parent] = event.Test
			}
			continue
		}
		if !packageNamePattern.MatchString(event.Package) || !testNamePattern.MatchString(event.Test) {
			continue
		}
		if event.Action != "pass" && event.Action != "fail" && event.Action != "skip" {
			continue
		}
		key := event.Package + "\x00" + event.Test
		result := TestResult{
			Package: event.Package,
			Name:    event.Test,
			Status:  event.Action,
			Elapsed: event.Elapsed,
		}
		if previous, exists := observed[key]; exists {
			result.Elapsed += previous.Elapsed
			if resultSeverity(previous.Status) > resultSeverity(result.Status) {
				result.Status = previous.Status
			}
		}
		observed[key] = result
	}
	for key, phase := range nestedFailures {
		if result, ok := observed[key]; ok && result.Status == "fail" {
			result.Phase = phase
			observed[key] = result
		}
	}

	summary := Summary{
		SchemaVersion: 1,
		Manifest:      manifest.Version,
		Suite:         suiteName,
		Tier:          suite.Tier,
		Valid:         true,
		Tests:         make([]TestResult, 0, len(observed)),
	}
	for _, result := range observed {
		summary.Tests = append(summary.Tests, result)
	}
	sort.Slice(summary.Tests, func(i, j int) bool {
		if summary.Tests[i].Package == summary.Tests[j].Package {
			return summary.Tests[i].Name < summary.Tests[j].Name
		}
		return summary.Tests[i].Package < summary.Tests[j].Package
	})

	exact := make(map[string]Expectation)
	wildcards := make(map[string]Expectation)
	for _, expectation := range suite.Expected {
		if expectation.Name == "*" {
			wildcards[expectation.Package] = expectation
			continue
		}
		exact[expectation.Package+"\x00"+expectation.Name] = expectation
	}

	wildcardCounts := make(map[string]int)
	for _, result := range summary.Tests {
		key := result.Package + "\x00" + result.Name
		expectation, expected := exact[key]
		if wildcard, wildcardExpected := wildcards[result.Package]; wildcardExpected {
			expectation = wildcard
			expected = true
			wildcardCounts[result.Package]++
		}
		if !expected {
			summary.Counts.Unexpected++
			summary.Problems = append(summary.Problems, "unexpected: "+result.Package+" "+result.Name)
		}
		switch result.Status {
		case "pass":
			summary.Counts.Passed++
		case "fail":
			summary.Counts.Failed++
			summary.Problems = append(summary.Problems, "failed: "+result.Package+" "+result.Name)
		case "skip":
			summary.Counts.Skipped++
			if !expected || !expectation.AllowSkip {
				summary.Problems = append(summary.Problems, "skipped: "+result.Package+" "+result.Name)
			}
		}
	}

	for key, expectation := range exact {
		if _, ok := observed[key]; !ok {
			summary.Counts.Missing++
			summary.Problems = append(summary.Problems, "missing: "+expectation.Package+" "+expectation.Name)
		}
	}
	for packageName, expectation := range wildcards {
		minimum := expectation.Minimum
		if minimum == 0 {
			minimum = 1
		}
		if wildcardCounts[packageName] < minimum {
			summary.Counts.Missing += minimum - wildcardCounts[packageName]
			summary.Problems = append(summary.Problems, fmt.Sprintf("missing: %s requires at least %d top-level tests", packageName, minimum))
		}
	}
	for packageName := range packageFailures {
		summary.Problems = append(summary.Problems, "package failed: "+packageName)
	}
	for key := range nestedSkips {
		parts := strings.SplitN(key, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		parentKey := parts[0] + "\x00" + parts[1]
		expectation, expected := exact[parentKey]
		if wildcard, wildcardExpected := wildcards[parts[0]]; wildcardExpected {
			expectation = wildcard
			expected = true
		}
		if expected && !expectation.AllowSkip {
			summary.Counts.Skipped++
			summary.Problems = append(summary.Problems, "nested skip: "+parts[0]+" "+parts[2])
		}
	}

	sort.Strings(summary.Problems)
	summary.Valid = len(summary.Problems) == 0
	if !summary.Valid {
		return summary, errors.New("test qualification failed")
	}
	return summary, nil
}

func resultSeverity(status string) int {
	switch status {
	case "fail":
		return 3
	case "skip":
		return 2
	case "pass":
		return 1
	default:
		return 0
	}
}

// SetMatrix adds bounded, allow-listed matrix metadata to a summary.
func SetMatrix(summary *Summary, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	if len(values) > 16 {
		return errors.New("too many matrix metadata entries")
	}
	summary.Matrix = make(map[string]string, len(values))
	for key, value := range values {
		pattern, ok := matrixPatterns[key]
		if !ok || !metadataPattern.MatchString(key) || !metadataPattern.MatchString(value) || !pattern.MatchString(value) {
			return errors.New("matrix metadata contains an invalid key or value")
		}
		summary.Matrix[key] = value
	}
	return nil
}

func readBounded(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("input exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func requireEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing manifest data: %w", err)
	}
	return errors.New("manifest contains more than one JSON document")
}
