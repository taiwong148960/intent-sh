package shelltest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const strictQualificationEnvironment = "INTENT_SH_CI_STRICT"
const executableCoverageEnvironment = "INTENT_SH_EXEC_COVERAGE_DIR"

// qualificationSkipf preserves optional local integration tests while making
// the same missing prerequisite a hard failure in repository-owned CI targets.
func qualificationSkipf(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv(strictQualificationEnvironment) == "1" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

func TestQualificationStrictFlagIsExplicit(t *testing.T) {
	t.Setenv(strictQualificationEnvironment, "")
	if qualificationIsStrict() {
		t.Fatal("empty strict qualification flag enabled strict mode")
	}
	t.Setenv(strictQualificationEnvironment, "1")
	if !qualificationIsStrict() {
		t.Fatal("strict qualification flag did not enable strict mode")
	}
	t.Setenv(strictQualificationEnvironment, "true")
	if qualificationIsStrict() {
		t.Fatal("non-canonical strict qualification value enabled strict mode")
	}
	coverageDirectory := t.TempDir()
	t.Setenv(executableCoverageEnvironment, coverageDirectory)
	if value, err := qualificationExecutableCoverageDirectory(); err != nil || value != coverageDirectory {
		t.Fatalf("valid executable coverage directory was rejected: %q %v", value, err)
	}
	t.Setenv(executableCoverageEnvironment, "relative")
	if _, err := qualificationExecutableCoverageDirectory(); err == nil {
		t.Fatal("relative executable coverage directory was accepted")
	}
	symlink := filepath.Join(t.TempDir(), "coverage-link")
	if err := os.Symlink(coverageDirectory, symlink); err != nil {
		t.Fatal(err)
	}
	t.Setenv(executableCoverageEnvironment, symlink)
	if _, err := qualificationExecutableCoverageDirectory(); err == nil {
		t.Fatal("symlink executable coverage directory was accepted")
	}
}

func qualificationIsStrict() bool {
	return os.Getenv(strictQualificationEnvironment) == "1"
}

func qualificationExecutableCoverageDirectory() (string, error) {
	value := os.Getenv(executableCoverageEnvironment)
	if value == "" {
		return "", nil
	}
	if len(value) > 500 || !filepath.IsAbs(value) || filepath.Clean(value) != value || value == string(filepath.Separator) {
		return "", errors.New("executable coverage directory must be a bounded absolute clean path")
	}
	info, err := os.Lstat(value)
	if err != nil {
		return "", errors.New("inspect executable coverage directory")
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 {
		return "", errors.New("executable coverage directory must be a private real directory")
	}
	return value, nil
}
