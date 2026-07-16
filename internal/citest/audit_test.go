package citest

import (
	"encoding/json"
	"strings"
	"testing"
)

const testPackage = "github.com/taiwong148960/intent-sh/internal/example"

func TestAuditAcceptsCompletePassingSuite(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("run", "TestExpected") + event("pass", "TestExpected") + packageEvent("pass")

	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if !summary.Valid || summary.Counts.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAuditRejectsSkippedExpectedTest(t *testing.T) {
	manifest := exactManifest("TestExpected")
	summary, err := Audit(manifest, "required", strings.NewReader(event("skip", "TestExpected")+packageEvent("pass")), 4096, 100)
	if err == nil {
		t.Fatal("Audit() unexpectedly succeeded")
	}
	if summary.Counts.Skipped != 1 || summary.Valid {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAuditRepeatedCaseRetainsWorstOutcome(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("skip", "TestExpected") + event("pass", "TestExpected") + packageEvent("pass")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err == nil {
		t.Fatal("Audit() accepted a skipped repetition followed by a pass")
	}
	if len(summary.Tests) != 1 || summary.Tests[0].Status != "skip" || summary.Tests[0].Elapsed != 0.02 {
		t.Fatalf("repeated summary = %#v", summary)
	}
}

func TestAuditRejectsNestedSkipUnderExpectedTest(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("run", "TestExpected") + event("skip", "TestExpected/missing-shell") + event("pass", "TestExpected") + packageEvent("pass")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err == nil {
		t.Fatal("Audit() unexpectedly succeeded")
	}
	if summary.Counts.Skipped != 1 || summary.Valid {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAuditAllowsDeclaredLocalSkip(t *testing.T) {
	manifest := exactManifest("TestExpected")
	suite := manifest.Suites["required"]
	suite.Tier = "local-optional"
	suite.Expected[0].AllowSkip = true
	manifest.Suites["required"] = suite

	summary, err := Audit(manifest, "required", strings.NewReader(event("skip", "TestExpected")+packageEvent("pass")), 4096, 100)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if !summary.Valid || summary.Counts.Skipped != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAuditIgnoresPassingNestedSubtests(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("run", "TestExpected") + event("pass", "TestExpected/nested") + event("pass", "TestExpected") + packageEvent("pass")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if len(summary.Tests) != 1 || summary.Tests[0].Name != "TestExpected" {
		t.Fatalf("tests = %#v", summary.Tests)
	}
}

func TestAuditRejectsMissingExpectedTest(t *testing.T) {
	manifest := exactManifest("TestExpected")
	summary, err := Audit(manifest, "required", strings.NewReader(packageEvent("pass")), 4096, 100)
	if err == nil {
		t.Fatal("Audit() unexpectedly succeeded")
	}
	if summary.Counts.Missing != 1 {
		t.Fatalf("missing = %d, want 1", summary.Counts.Missing)
	}
}

func TestAuditRejectsRenamedOrUnexpectedTest(t *testing.T) {
	manifest := exactManifest("TestOldName")
	summary, err := Audit(manifest, "required", strings.NewReader(event("pass", "TestNewName")+packageEvent("pass")), 4096, 100)
	if err == nil {
		t.Fatal("Audit() unexpectedly succeeded")
	}
	if summary.Counts.Missing != 1 || summary.Counts.Unexpected != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAuditRejectsFailedTestAndPackage(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("fail", "TestExpected/linux/process-signal") + event("fail", "TestExpected") + packageEvent("fail")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err == nil {
		t.Fatal("Audit() unexpectedly succeeded")
	}
	if summary.Counts.Failed != 1 || len(summary.Problems) != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	if len(summary.Tests) != 1 || summary.Tests[0].Phase != "TestExpected/linux/process-signal" {
		t.Fatalf("failure phase = %#v", summary.Tests)
	}
}

func TestAuditAcceptsPackageWildcardAndEnforcesMinimum(t *testing.T) {
	manifest := Manifest{Version: ManifestVersion, Suites: map[string]Suite{
		"required": {
			Tier:     "required",
			Expected: []Expectation{{Package: testPackage, Name: "*", Minimum: 2}},
		},
	}}
	stream := event("pass", "TestOne") + event("pass", "TestTwo") + packageEvent("pass")
	if _, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100); err != nil {
		t.Fatalf("Audit() error = %v", err)
	}

	summary, err := Audit(manifest, "required", strings.NewReader(event("pass", "TestOne")+packageEvent("pass")), 4096, 100)
	if err == nil || summary.Counts.Missing != 1 {
		t.Fatalf("summary = %#v, error = %v", summary, err)
	}
}

func TestAuditBoundsInputAndEventCount(t *testing.T) {
	manifest := exactManifest("TestExpected")
	if _, err := Audit(manifest, "required", strings.NewReader(strings.Repeat("x", 65)), 64, 100); err == nil {
		t.Fatal("Audit() accepted oversized input")
	}
	stream := event("run", "TestExpected") + event("pass", "TestExpected")
	if _, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 1); err == nil {
		t.Fatal("Audit() accepted too many events")
	}
	if _, err := Audit(manifest, "required", strings.NewReader("{not-json}\n"), 4096, 100); err == nil {
		t.Fatal("Audit() accepted malformed input")
	}
}

func TestAuditSummaryOmitsArbitraryTerminalOutput(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := `{"Action":"output","Package":"` + testPackage + `","Test":"TestExpected","Output":"\u001b]0;PRIVATE_VALUE\u0007"}` + "\n" +
		event("pass", "TestExpected") + packageEvent("pass")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "PRIVATE_VALUE") || strings.ContainsRune(string(encoded), '\x1b') {
		t.Fatalf("summary retained arbitrary terminal output: %s", encoded)
	}
}

func TestAuditRejectsUnsafeFailurePhaseFromEvidence(t *testing.T) {
	manifest := exactManifest("TestExpected")
	stream := event("fail", "TestExpected/private value") + event("fail", "TestExpected") + packageEvent("fail")
	summary, err := Audit(manifest, "required", strings.NewReader(stream), 4096, 100)
	if err == nil {
		t.Fatal("failed test unexpectedly passed")
	}
	encoded, marshalErr := json.Marshal(summary)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(encoded), "private value") {
		t.Fatalf("unsafe nested phase entered evidence: %s", encoded)
	}
}

func TestLoadManifestRejectsUnknownFieldsAndMixedExpectationModes(t *testing.T) {
	unknown := `{"version":1,"suites":{},"secret":"value"}`
	if _, err := LoadManifest(strings.NewReader(unknown), 4096); err == nil {
		t.Fatal("LoadManifest() accepted an unknown field")
	}
	mixed := Manifest{Version: ManifestVersion, Suites: map[string]Suite{
		"required": {
			Tier: "required",
			Expected: []Expectation{
				{Package: testPackage, Name: "*"},
				{Package: testPackage, Name: "TestExact"},
			},
		},
	}}
	if err := ValidateManifest(mixed); err == nil {
		t.Fatal("ValidateManifest() accepted wildcard and exact expectations for one package")
	}
}

func exactManifest(testName string) Manifest {
	return Manifest{Version: ManifestVersion, Suites: map[string]Suite{
		"required": {
			Tier:     "required",
			Expected: []Expectation{{Package: testPackage, Name: testName}},
		},
	}}
}

func event(action, testName string) string {
	return `{"Action":"` + action + `","Package":"` + testPackage + `","Test":"` + testName + `","Elapsed":0.01}` + "\n"
}

func packageEvent(action string) string {
	return `{"Action":"` + action + `","Package":"` + testPackage + `","Elapsed":0.02}` + "\n"
}
