package citest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const repositoryModule = "github.com/taiwong148960/intent-sh"

func TestRepositoryManifestInventoriesEveryTopLevelTest(t *testing.T) {
	root := repositoryRoot(t)
	manifest := loadRepositoryManifest(t, root)
	tests := repositoryTests(t, root)

	for _, test := range tests {
		matched := false
		for _, suite := range manifest.Suites {
			for _, expectation := range suite.Expected {
				if expectation.Package == test.Package && (expectation.Name == "*" || expectation.Name == test.Name) {
					matched = true
				}
			}
		}
		if !matched {
			t.Errorf("top-level test is absent from the CI manifest: %s %s", test.Package, test.Name)
		}
	}

	known := make(map[string]bool, len(tests))
	for _, test := range tests {
		known[test.Package+"\x00"+test.Name] = true
	}
	for suiteName, suite := range manifest.Suites {
		for _, expectation := range suite.Expected {
			if expectation.Name != "*" && !known[expectation.Package+"\x00"+expectation.Name] {
				t.Errorf("suite %s refers to a missing or renamed test: %s %s", suiteName, expectation.Package, expectation.Name)
			}
		}
	}
}

func TestRequiredBroadSuiteExcludesDedicatedIntegrationPackages(t *testing.T) {
	manifest := loadRepositoryManifest(t, repositoryRoot(t))
	unit, ok := manifest.Suites["unit"]
	if !ok {
		t.Fatal("manifest has no required unit suite")
	}
	for _, expectation := range unit.Expected {
		if expectation.Package == repositoryModule+"/internal/shelltest" || expectation.Package == repositoryModule+"/internal/smoketest" {
			t.Errorf("broad unit suite selects dedicated integration package %s", expectation.Package)
		}
	}
	for _, dedicated := range []string{"native-pty", "tmux", "external-ssh", "real-provider"} {
		if _, ok := manifest.Suites[dedicated]; !ok {
			t.Errorf("manifest has no dedicated %s suite", dedicated)
		}
	}
}

type repositoryTest struct {
	Package string
	Name    string
}

func repositoryTests(t *testing.T, root string) []repositoryTest {
	t.Helper()
	var tests []repositoryTest
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relativeDirectory, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		packageName := repositoryModule
		if relativeDirectory != "." {
			packageName += "/" + filepath.ToSlash(relativeDirectory)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !testNamePattern.MatchString(function.Name.Name) {
				continue
			}
			tests = append(tests, repositoryTest{Package: packageName, Name: function.Name.Name})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inventory repository tests: %v", err)
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].Package == tests[j].Package {
			return tests[i].Name < tests[j].Name
		}
		return tests[i].Package < tests[j].Package
	})
	return tests
}

func loadRepositoryManifest(t *testing.T, root string) Manifest {
	t.Helper()
	file, err := os.Open(filepath.Join(root, ".github", "ci", "test-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	manifest, err := LoadManifest(file, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
