package citest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const MaxWorkflowBytes = 1 << 20

var (
	immutableActionPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[a-f0-9]{40}$`)
	writePermissionPattern = regexp.MustCompile(`(?m)^[ \t]{2,}[A-Za-z0-9_-]+:[ \t]*write[ \t]*$`)
	pullTargetPattern      = regexp.MustCompile(`(?m)^[ \t]*pull_request_target[ \t]*:`)
	continueErrorPattern   = regexp.MustCompile(`(?m)^[ \t]*(?:-[ \t]*)?continue-on-error[ \t]*:`)
	workflowKeyPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	secretContextPattern   = regexp.MustCompile(`secrets[ \t]*(?:\.|\[)`)
)

func ValidateWorkflowPolicy(reader io.Reader) error {
	data, err := readBounded(reader, MaxWorkflowBytes)
	if err != nil {
		return errors.New("read bounded workflow")
	}
	text := string(data)
	if pullTargetPattern.MatchString(text) {
		return errors.New("workflow contains pull_request_target")
	}
	if continueErrorPattern.MatchString(text) {
		return errors.New("workflow contains continue-on-error")
	}
	if !regexp.MustCompile(`(?m)^permissions:\s*\n\s{2}contents:\s*read\s*$`).MatchString(text) {
		return errors.New("workflow must declare read-only top-level contents permission")
	}
	if writePermissionPattern.MatchString(text) || strings.Contains(text, "permissions: write-all") {
		return errors.New("workflow grants a write permission")
	}
	if secretContextPattern.MatchString(text) {
		return errors.New("repository workflows must not reference secret contexts")
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNumber := 0
	checkoutPending := 0
	permissionsIndent := -1
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		leading := rawLine[:len(rawLine)-len(strings.TrimLeft(rawLine, " \t"))]
		if strings.Contains(leading, "\t") {
			return fmt.Errorf("workflow indentation near line %d uses a tab", lineNumber)
		}
		indent := len(leading)
		if permissionsIndent >= 0 && indent <= permissionsIndent {
			permissionsIndent = -1
		}
		if permissionsIndent >= 0 {
			key, value, ok := strings.Cut(line, ":")
			value = strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
			if !ok || indent != permissionsIndent+2 || !workflowKeyPattern.MatchString(key) || (value != "read" && value != "none") {
				return fmt.Errorf("permissions near line %d are not literal read/none values", lineNumber)
			}
			continue
		}
		if strings.HasPrefix(line, "permissions:") {
			if line != "permissions:" {
				return fmt.Errorf("inline permissions near line %d are forbidden", lineNumber)
			}
			permissionsIndent = indent
			continue
		}
		if checkoutPending > 0 {
			checkoutPending--
			if line == "persist-credentials: false" {
				checkoutPending = 0
			}
			if checkoutPending == 0 && line != "persist-credentials: false" {
				return fmt.Errorf("checkout near line %d does not disable credential persistence", lineNumber)
			}
		}
		if strings.Contains(line, "uses:") && !strings.HasPrefix(line, "uses:") && !strings.HasPrefix(line, "- uses:") {
			return fmt.Errorf("unsupported uses syntax near line %d", lineNumber)
		}
		if !strings.HasPrefix(line, "uses:") && !strings.HasPrefix(line, "- uses:") {
			continue
		}
		usesLine := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		value := strings.TrimSpace(strings.TrimPrefix(usesLine, "uses:"))
		value, _, _ = strings.Cut(value, " #")
		if strings.HasPrefix(value, "./") {
			continue
		}
		if !immutableActionPattern.MatchString(value) {
			return fmt.Errorf("third-party action near line %d is not pinned to a commit", lineNumber)
		}
		if strings.HasPrefix(value, "actions/checkout@") {
			checkoutPending = 8
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.New("scan bounded workflow")
	}
	if checkoutPending > 0 {
		return errors.New("checkout at end of workflow does not disable credential persistence")
	}
	return nil
}
