package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taiwong148960/intent-sh/internal/coveragequal"
)

func main() {
	profilePath := flag.String("profile", "", "merged atomic coverage profile")
	policyPath := flag.String("policy", "", "checked-in coverage policy")
	summaryPath := flag.String("summary", "", "sanitized JSON summary")
	flag.Parse()
	if flag.NArg() != 0 || !safePath(*profilePath) || !safePath(*policyPath) || !safePath(*summaryPath) {
		fatal(errors.New("profile, policy, and summary must be bounded absolute clean paths"))
	}
	policyFile, err := os.Open(*policyPath)
	if err != nil {
		fatal(errors.New("open coverage policy"))
	}
	policy, policyErr := coveragequal.ParsePolicy(policyFile)
	closePolicyErr := policyFile.Close()
	if policyErr != nil || closePolicyErr != nil {
		fatal(errors.New("parse coverage policy"))
	}
	profileFile, err := os.Open(*profilePath)
	if err != nil {
		fatal(errors.New("open merged coverage profile"))
	}
	result, evaluationErr := coveragequal.EvaluateProfile(profileFile, policy)
	closeProfileErr := profileFile.Close()

	status := "pass"
	if evaluationErr != nil || closeProfileErr != nil {
		status = "fail"
	}
	summary := struct {
		Schema     int     `json:"schema"`
		Status     string  `json:"status"`
		Percent    float64 `json:"percent"`
		Covered    int     `json:"covered_statements"`
		Total      int     `json:"total_statements"`
		Floor      float64 `json:"floor"`
		Tolerance  float64 `json:"tolerance"`
		Exclusions int     `json:"documented_exclusions"`
	}{1, status, result.Percent, result.CoveredStatements, result.TotalStatements, policy.Floor, policy.Tolerance, len(policy.ExcludePrefixes)}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fatal(errors.New("encode coverage summary"))
	}
	data = append(data, '\n')
	if err := os.WriteFile(*summaryPath, data, 0o600); err != nil {
		fatal(errors.New("write coverage summary"))
	}
	fmt.Printf("aggregate source coverage %.2f%% (%d/%d statements); policy threshold %.2f%%\n", result.Percent, result.CoveredStatements, result.TotalStatements, result.Threshold)
	if evaluationErr != nil || closeProfileErr != nil {
		fatal(errors.New("aggregate source coverage did not satisfy the checked-in policy"))
	}
}

func safePath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && value != string(filepath.Separator) && len(value) <= 500
}

func fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
