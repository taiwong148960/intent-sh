// Package artifactqual validates release-path intent-sh executables without
// running artifacts built for another macOS architecture.
package artifactqual

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/macho"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

const MaxArtifactBytes int64 = 128 << 20

type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

func (target Target) Filename() string {
	return "intent-sh-" + target.GOOS + "-" + target.GOARCH
}

var SupportedTargets = []Target{
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
}

type Report struct {
	Target
	Filename        string `json:"filename"`
	Format          string `json:"format"`
	SHA256          string `json:"sha256"`
	Size            int64  `json:"size"`
	Module          string `json:"module"`
	AdapterProtocol string `json:"adapter_protocol"`
}

func Inspect(path string, target Target) (Report, error) {
	report := Report{Target: target, Filename: target.Filename(), AdapterProtocol: "2"}
	if !supportedTarget(target) {
		return Report{}, fmt.Errorf("unsupported artifact target %s/%s", target.GOOS, target.GOARCH)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Report{}, fmt.Errorf("inspect artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return Report{}, errors.New("artifact must be a regular executable file")
	}
	if info.Size() <= 0 || info.Size() > MaxArtifactBytes {
		return Report{}, errors.New("artifact size is outside the qualification boundary")
	}
	file, err := os.Open(path)
	if err != nil {
		return Report{}, fmt.Errorf("open artifact: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, MaxArtifactBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(data)) != info.Size() {
		return Report{}, errors.New("read complete bounded artifact")
	}
	digest := sha256.Sum256(data)
	report.SHA256 = hex.EncodeToString(digest[:])
	report.Size = info.Size()

	binary, err := macho.Open(path)
	if err != nil {
		return Report{}, fmt.Errorf("open Mach-O artifact: %w", err)
	}
	defer binary.Close()
	if binary.Type != macho.TypeExec || !machoCPUIs(binary.Cpu, target.GOARCH) {
		return Report{}, errors.New("Mach-O executable architecture does not match its target")
	}
	report.Format = "Mach-O"

	build, err := buildinfo.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read Go build metadata: %w", err)
	}
	report.Module = build.Path
	if report.Module != "github.com/taiwong148960/intent-sh/cmd/intent-sh" {
		return Report{}, errors.New("artifact Go module path is not intent-sh")
	}
	settings := make(map[string]string, len(build.Settings))
	for _, setting := range build.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != target.GOOS || settings["GOARCH"] != target.GOARCH || settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" {
		return Report{}, errors.New("artifact build metadata omitted its target, static-build, or trimpath contract")
	}
	for _, marker := range [][]byte{
		[]byte("# intent-sh Bash adapter (protocol 2)"),
		[]byte("# intent-sh Zsh adapter (protocol 2)"),
		[]byte("__intent_sh_protocol_version=2"),
	} {
		if !bytes.Contains(data, marker) {
			return Report{}, errors.New("artifact omitted an embedded adapter protocol marker")
		}
	}
	return report, nil
}

func NativeTarget() Target {
	return Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

func supportedTarget(target Target) bool {
	for _, supported := range SupportedTargets {
		if target == supported {
			return true
		}
	}
	return false
}

func machoCPUIs(cpu macho.Cpu, architecture string) bool {
	return (architecture == "amd64" && cpu == macho.CpuAmd64) || (architecture == "arm64" && cpu == macho.CpuArm64)
}
