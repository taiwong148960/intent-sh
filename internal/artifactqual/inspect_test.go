package artifactqual

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInspectNativeReproducibleArtifact(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("artifact qualification requires macOS")
	}
	root := filepath.Clean(filepath.Join(repositoryDirectory(t), "..", ".."))
	path := filepath.Join(t.TempDir(), NativeTarget().Filename())
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-buildid=", "-o", path, "./cmd/intent-sh")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native artifact: %v: %s", err, output)
	}
	report, err := Inspect(path, NativeTarget())
	if err != nil {
		t.Fatalf("inspect native artifact: %v", err)
	}
	if report.Format == "" || len(report.SHA256) != 64 || report.AdapterProtocol != "2" {
		t.Fatalf("incomplete artifact report: %#v", report)
	}

	nonExecutable := filepath.Join(t.TempDir(), "not-executable")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nonExecutable, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(nonExecutable, NativeTarget()); err == nil {
		t.Fatal("non-executable artifact was accepted")
	}
	symlink := filepath.Join(t.TempDir(), "symlink")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(symlink, NativeTarget()); err == nil {
		t.Fatal("symlink artifact was accepted")
	}
}

func TestInspectRejectsWrongTargetAndTruncatedInput(t *testing.T) {
	truncated := filepath.Join(t.TempDir(), "intent-sh-darwin-amd64")
	if err := os.WriteFile(truncated, []byte("not an executable artifact"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(truncated, Target{GOOS: "darwin", GOARCH: "amd64"}); err == nil {
		t.Fatal("truncated artifact was accepted")
	}
	if _, err := Inspect(truncated, Target{GOOS: "invalid", GOARCH: "amd64"}); err == nil {
		t.Fatal("unsupported target was accepted")
	}
}

func repositoryDirectory(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return working
}
