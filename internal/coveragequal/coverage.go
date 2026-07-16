// Package coveragequal enforces the checked-in aggregate source coverage
// policy without sending source or profiles to a third party.
package coveragequal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxCoverageLines = 2_000_000

type Policy struct {
	Floor           float64
	Tolerance       float64
	ExcludePrefixes []string
}

type Result struct {
	CoveredStatements int
	TotalStatements   int
	Percent           float64
	Threshold         float64
}

func ParsePolicy(reader io.Reader) (Policy, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, 32<<10))
	policy := Policy{}
	seen := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || seen[key] {
			return Policy{}, errors.New("coverage policy contains a malformed or duplicate field")
		}
		seen[key] = true
		switch key {
		case "COVERAGE_FLOOR":
			parsed, err := parsePercentage(value)
			if err != nil {
				return Policy{}, err
			}
			policy.Floor = parsed
		case "COVERAGE_TOLERANCE":
			parsed, err := parsePercentage(value)
			if err != nil {
				return Policy{}, err
			}
			policy.Tolerance = parsed
		case "EXCLUDE_PREFIXES":
			if value == "" {
				continue
			}
			for _, prefix := range strings.Split(value, ",") {
				if !safePackagePrefix(prefix) {
					return Policy{}, errors.New("coverage exclusion is not a bounded module package prefix")
				}
				policy.ExcludePrefixes = append(policy.ExcludePrefixes, prefix)
			}
		default:
			return Policy{}, errors.New("coverage policy contains an unknown field")
		}
	}
	if err := scanner.Err(); err != nil {
		return Policy{}, errors.New("read bounded coverage policy")
	}
	if !seen["COVERAGE_FLOOR"] || !seen["COVERAGE_TOLERANCE"] || !seen["EXCLUDE_PREFIXES"] || policy.Tolerance > policy.Floor {
		return Policy{}, errors.New("coverage policy is incomplete or internally inconsistent")
	}
	return policy, nil
}

func EvaluateProfile(reader io.Reader, policy Policy) (Result, error) {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 64<<10)
	scanner.Buffer(buffer, 1<<20)
	if !scanner.Scan() || scanner.Text() != "mode: atomic" {
		return Result{}, errors.New("coverage profile must use atomic mode")
	}
	result := Result{Threshold: policy.Floor - policy.Tolerance}
	lineCount := 1
	for scanner.Scan() {
		lineCount++
		if lineCount > maxCoverageLines {
			return Result{}, errors.New("coverage profile exceeded its line bound")
		}
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return Result{}, errors.New("coverage profile contains a malformed record")
		}
		location := fields[0]
		colon := strings.LastIndex(location, ":")
		if colon <= 0 {
			return Result{}, errors.New("coverage profile contains a malformed source location")
		}
		filename := location[:colon]
		if excluded(filename, policy.ExcludePrefixes) {
			continue
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil || statements < 0 || statements > 1_000_000 {
			return Result{}, errors.New("coverage profile contains an invalid statement count")
		}
		count, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			return Result{}, errors.New("coverage profile contains an invalid execution count")
		}
		result.TotalStatements += statements
		if count > 0 {
			result.CoveredStatements += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, errors.New("read bounded coverage profile")
	}
	if result.TotalStatements == 0 {
		return Result{}, errors.New("coverage profile contains no policy-scoped statements")
	}
	result.Percent = 100 * float64(result.CoveredStatements) / float64(result.TotalStatements)
	if result.Percent+1e-9 < result.Threshold {
		return result, fmt.Errorf("aggregate coverage %.2f%% is below the checked-in %.2f%% threshold", result.Percent, result.Threshold)
	}
	return result, nil
}

func parsePercentage(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || parsed > 100 {
		return 0, errors.New("coverage policy percentage is invalid")
	}
	return parsed, nil
}

func safePackagePrefix(value string) bool {
	if len(value) == 0 || len(value) > 300 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._-/", r) {
			continue
		}
		return false
	}
	return true
}

func excluded(filename string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(filename, prefix+"/") {
			return true
		}
	}
	return false
}
