// Package contextinfo constructs the explicit, minimal model context.
package contextinfo

import (
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

var ToolAllowlist = []string{
	"awk", "curl", "docker", "fd", "find", "git", "grep", "jq", "kubectl",
	"lsof", "ps", "rg", "sed", "ss", "wget",
}

type Environment struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	Shell          string   `json:"shell"`
	ShellVersion   string   `json:"shellVersion"`
	CWD            string   `json:"cwd"`
	Remote         bool     `json:"remote"`
	Locale         string   `json:"locale"`
	AvailableTools []string `json:"availableTools"`
}

type Builder struct {
	GOOS     string
	GOARCH   string
	Getenv   func(string) string
	LookPath func(string) (string, error)
}

func NewBuilder() Builder {
	return Builder{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Getenv: os.Getenv, LookPath: exec.LookPath}
}

func (b Builder) Build(shell, shellVersion, cwd string) Environment {
	getenv := b.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	lookPath := b.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	tools := make([]string, 0, len(ToolAllowlist))
	for _, tool := range ToolAllowlist {
		if _, err := lookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}
	sort.Strings(tools)
	return Environment{
		OS:             b.GOOS,
		Arch:           b.GOARCH,
		Shell:          boundedLine(shell, 32),
		ShellVersion:   boundedLine(shellVersion, 64),
		CWD:            boundedLine(cwd, 4096),
		Remote:         getenv("SSH_CONNECTION") != "" || getenv("SSH_CLIENT") != "" || getenv("SSH_TTY") != "",
		Locale:         locale(getenv),
		AvailableTools: tools,
	}
}

func locale(getenv func(string) string) string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := boundedLine(getenv(key), 64); value != "" {
			return value
		}
	}
	return ""
}

func boundedLine(value string, max int) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	if len(value) > max {
		return value[:max]
	}
	return value
}
