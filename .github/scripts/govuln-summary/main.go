package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
)

const (
	maxInputBytes = 256 << 20
	maxMessages   = 1_000_000
)

var safeMetadata = regexp.MustCompile(`^[A-Za-z0-9._+:/TZ -]{0,200}$`)

type message struct {
	Config *struct {
		ScannerName    string `json:"scanner_name"`
		ScannerVersion string `json:"scanner_version"`
		DB             string `json:"db"`
		DBModified     string `json:"db_last_modified"`
		GoVersion      string `json:"go_version"`
	} `json:"config"`
	Finding *struct {
		OSV          string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
	} `json:"finding"`
}

func main() {
	limited := &io.LimitedReader{R: os.Stdin, N: maxInputBytes}
	decoder := json.NewDecoder(bufio.NewReaderSize(limited, 64<<10))
	var config *struct {
		ScannerName    string `json:"scanner_name"`
		ScannerVersion string `json:"scanner_version"`
		DB             string `json:"db"`
		DBModified     string `json:"db_last_modified"`
		GoVersion      string `json:"go_version"`
	}
	findings := map[string]string{}
	for count := 0; ; count++ {
		if count >= maxMessages {
			fatal(errors.New("govulncheck result exceeded its message bound"))
		}
		var value message
		err := decoder.Decode(&value)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fatal(errors.New("govulncheck returned malformed JSON"))
		}
		if value.Config != nil {
			if config != nil {
				fatal(errors.New("govulncheck returned duplicate configuration metadata"))
			}
			config = value.Config
		}
		if value.Finding != nil {
			if !regexp.MustCompile(`^GO-[0-9]{4}-[0-9]{4,}$`).MatchString(value.Finding.OSV) || !safeMetadata.MatchString(value.Finding.FixedVersion) {
				fatal(errors.New("govulncheck returned unsafe finding metadata"))
			}
			findings[value.Finding.OSV] = value.Finding.FixedVersion
		}
	}
	if limited.N == 0 {
		var extra [1]byte
		n, err := os.Stdin.Read(extra[:])
		if n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
			fatal(errors.New("govulncheck result exceeded its byte bound"))
		}
	}
	if config == nil || !safeMetadata.MatchString(config.ScannerName) || !safeMetadata.MatchString(config.ScannerVersion) || !safeMetadata.MatchString(config.DB) || !safeMetadata.MatchString(config.DBModified) || !safeMetadata.MatchString(config.GoVersion) {
		fatal(errors.New("govulncheck omitted bounded scanner or database metadata"))
	}
	ids := make([]string, 0, len(findings))
	for id := range findings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := struct {
		Scanner    string   `json:"scanner"`
		Version    string   `json:"version"`
		Database   string   `json:"database"`
		DBModified string   `json:"database_modified"`
		GoVersion  string   `json:"go_version"`
		Status     string   `json:"status"`
		Findings   []string `json:"findings"`
	}{config.ScannerName, config.ScannerVersion, config.DB, config.DBModified, config.GoVersion, "pass", ids}
	if len(ids) > 0 {
		result.Status = "fail"
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(errors.New("encode vulnerability summary"))
	}
	if len(ids) > 0 {
		fatal(fmt.Errorf("govulncheck reported %d reachable vulnerability identifiers", len(ids)))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
