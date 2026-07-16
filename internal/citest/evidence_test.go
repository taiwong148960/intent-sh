package citest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceOmitsAdversarialSensitiveMaterial(t *testing.T) {
	markers := []string{
		"prompt=delete the customer database",
		"generated-command=rm -rf customer-data",
		"raw-pty=\x1b]52;clipboard-value\x07",
		"environment=AWS_SECRET_ACCESS_KEY_VALUE",
		"history=private shell history line",
		"ssh-private=-----BEGIN OPENSSH PRIVATE KEY-----",
		"provider-credential=sk-provider-token-value",
	}
	manifest := exactManifest("TestExpected")
	var stream strings.Builder
	for _, marker := range markers {
		payload, err := json.Marshal(map[string]any{
			"Action":  "output",
			"Package": testPackage,
			"Test":    "TestExpected",
			"Output":  marker,
		})
		if err != nil {
			t.Fatal(err)
		}
		stream.Write(payload)
		stream.WriteByte('\n')
	}
	stream.WriteString(event("pass", "TestExpected"))
	stream.WriteString(packageEvent("pass"))

	summary, err := Audit(manifest, "required", strings.NewReader(stream.String()), 16<<10, 100)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range markers {
		if strings.Contains(string(encoded), marker) {
			t.Fatalf("sanitized evidence retained sensitive marker %q", marker)
		}
	}
}

func TestEvidenceMatrixRejectsUnknownOrCredentialShapedMetadata(t *testing.T) {
	for _, values := range []map[string]string{
		{"environment": "production"},
		{"provider": "sk-provider-token-value"},
		{"go": "AWS_SECRET_ACCESS_KEY_VALUE"},
		{"seed": "history-line"},
		{"target": "user@private-host"},
	} {
		summary := Summary{}
		if err := SetMatrix(&summary, values); err == nil {
			t.Fatalf("SetMatrix() accepted unsafe metadata %#v", values)
		}
	}

	summary := Summary{}
	if err := SetMatrix(&summary, map[string]string{
		"os": "linux", "arch": "arm64", "go": "go1.25.1", "bash": "5.3.15(1)-release", "provider": "both", "target": "external",
	}); err != nil {
		t.Fatalf("SetMatrix() rejected declared metadata: %v", err)
	}
}

func TestWriteSummaryFileRejectsLinksAndPublishesPrivateAtomicJSON(t *testing.T) {
	directory := t.TempDir()
	secretPath := filepath.Join(directory, "private-key")
	if err := os.WriteFile(secretPath, []byte("PRIVATE_KEY_MATERIAL"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(directory, "summary.json")
	if err := os.Symlink(secretPath, outputPath); err != nil {
		t.Fatal(err)
	}
	if err := WriteSummaryFile(outputPath, Summary{SchemaVersion: 1}); err == nil {
		t.Fatal("WriteSummaryFile() followed an existing symlink")
	}
	secret, err := os.ReadFile(secretPath)
	if err != nil || string(secret) != "PRIVATE_KEY_MATERIAL" {
		t.Fatal("symlink destination changed")
	}
	if err := os.Remove(outputPath); err != nil {
		t.Fatal(err)
	}

	summary := Summary{SchemaVersion: 1, Suite: "unit", Tier: "required", Valid: true}
	if err := WriteSummaryFile(outputPath, summary); err != nil {
		t.Fatalf("WriteSummaryFile() error = %v", err)
	}
	info, err := os.Lstat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("summary mode = %v", info.Mode())
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE_KEY_MATERIAL") || !strings.Contains(string(data), `"suite":"unit"`) {
		t.Fatalf("unexpected summary content: %s", data)
	}
}
