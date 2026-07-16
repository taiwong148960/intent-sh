package coveragequal

import (
	"strings"
	"testing"
)

func TestParsePolicyAndEvaluateProfile(t *testing.T) {
	policy, err := ParsePolicy(strings.NewReader("COVERAGE_FLOOR=80\nCOVERAGE_TOLERANCE=1\nEXCLUDE_PREFIXES=example/internal/testonly\n"))
	if err != nil {
		t.Fatal(err)
	}
	profile := "mode: atomic\n" +
		"example/main.go:1.1,2.2 8 1\n" +
		"example/main.go:3.1,4.2 2 0\n" +
		"example/internal/testonly/helper.go:1.1,2.2 100 0\n" +
		"example/internal/testonly-adjacent/helper.go:1.1,2.2 1 1\n"
	result, err := EvaluateProfile(strings.NewReader(profile), policy)
	if err != nil {
		t.Fatal(err)
	}
	if result.Percent != 100*9.0/11.0 || result.CoveredStatements != 9 || result.TotalStatements != 11 || result.Threshold != 79 {
		t.Fatalf("coverage result = %#v", result)
	}
}

func TestCoveragePolicyFailsClosed(t *testing.T) {
	for _, value := range []string{
		"", "COVERAGE_FLOOR=80\n", "COVERAGE_FLOOR=bad\nCOVERAGE_TOLERANCE=1\nEXCLUDE_PREFIXES=\n",
		"COVERAGE_FLOOR=80\nCOVERAGE_FLOOR=80\nCOVERAGE_TOLERANCE=1\nEXCLUDE_PREFIXES=\n",
		"COVERAGE_FLOOR=80\nCOVERAGE_TOLERANCE=1\nEXCLUDE_PREFIXES=../../secret\n",
		"COVERAGE_FLOOR=80\nCOVERAGE_TOLERANCE=1\nEXCLUDE_PREFIXES=example/internal/testonly/\n",
	} {
		if _, err := ParsePolicy(strings.NewReader(value)); err == nil {
			t.Fatalf("malformed policy was accepted: %q", value)
		}
	}
	policy := Policy{Floor: 90, Tolerance: 0}
	for _, profile := range []string{
		"", "mode: set\nexample/main.go:1.1,2.2 1 1\n", "mode: atomic\nraw secret\n",
		"mode: atomic\nexample/main.go:1.1,2.2 1 invalid\n",
	} {
		if _, err := EvaluateProfile(strings.NewReader(profile), policy); err == nil {
			t.Fatalf("malformed profile was accepted: %q", profile)
		}
	}
	if result, err := EvaluateProfile(strings.NewReader("mode: atomic\nexample/main.go:1.1,2.2 10 0\n"), policy); err == nil || result.Percent != 0 {
		t.Fatal("coverage decrease did not fail the policy")
	}
}
