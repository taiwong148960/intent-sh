package citest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWorkflowPolicyRejectsPrivilegeAndSupplyChainHazards(t *testing.T) {
	valid := `name: test
on:
  workflow_dispatch:
permissions:
  contents: read
jobs:
  test:
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7
        with:
          persist-credentials: false
`
	if err := ValidateWorkflowPolicy(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid workflow rejected: %v", err)
	}
	for _, mutation := range []string{
		strings.Replace(valid, "workflow_dispatch:", "pull_request_target:", 1),
		strings.Replace(valid, "workflow_dispatch:", "pull_request_target :", 1),
		strings.Replace(valid, "contents: read", "contents: write", 1),
		valid + "  unsafe:\n    permissions: {contents: write}\n",
		strings.Replace(valid, "9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0", "v7", 1),
		strings.Replace(valid, "persist-credentials: false", "persist-credentials: true", 1),
		valid + "      - run: echo ${{ secrets.TOKEN }}\n" + "  pull_request:\n",
		valid + "      - run: echo ${{ secrets['TOKEN'] }}\n",
		valid + "      - continue-on-error: ${{ true }}\n        run: false\n",
	} {
		if err := ValidateWorkflowPolicy(strings.NewReader(mutation)); err == nil {
			t.Fatalf("unsafe workflow mutation was accepted:\n%s", mutation)
		}
	}
}

func TestRepositoryWorkflowsSatisfyReadOnlyImmutablePolicy(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate workflow policy test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	ymlPaths, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal("locate repository workflows")
	}
	yamlPaths, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yaml"))
	if err != nil {
		t.Fatal("locate repository workflows")
	}
	paths := append(ymlPaths, yamlPaths...)
	if len(paths) == 0 {
		t.Fatal("locate repository workflows")
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			validationErr := ValidateWorkflowPolicy(file)
			closeErr := file.Close()
			if validationErr != nil || closeErr != nil {
				t.Fatalf("workflow policy failed: %v", validationErr)
			}
		})
	}
}

func TestTrustedManualWorkflowKeepsCredentialsBehindProtectedBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "trusted-manual.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{
		"workflow_dispatch:",
		"runs-on: [self-hosted, intent-sh-trusted]",
		"environment: trusted-qualification",
		"persist-credentials: false",
		"make real-provider-test",
		"make external-ssh-test",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("trusted manual workflow is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"pull_request:",
		"pull_request_target:",
		"push:",
		"schedule:",
		"secrets.",
		"upload-artifact",
		"INTENT_SH_TEST_SSH_CONFIG",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("trusted manual workflow contains forbidden boundary %q", forbidden)
		}
	}
}
