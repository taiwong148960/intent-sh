// Package keyprobe implements the explicit, bounded controlling-terminal key
// delivery diagnostic. It never reads ordinary stdin or invokes a provider.
package keyprobe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/taiwong148960/intent-sh/internal/keychord"
)

const (
	CheckTTY     = "terminal.keys.tty"
	CheckRewrite = "terminal.keys.rewrite"
	CheckUndo    = "terminal.keys.undo"
	CheckEnter   = "terminal.keys.enter"
	CheckCancel  = "terminal.keys.cancel"
	CheckRestore = "terminal.keys.restore"

	DefaultPerKeyTimeout = 10 * time.Second
	DefaultMaxInputBytes = 8
)

var (
	ErrTimeout      = errors.New("key delivery timed out")
	ErrTooManyBytes = errors.New("key delivery exceeded the byte limit")
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

type Check struct {
	Status   Status
	ID       string
	Detail   string
	Guidance string
}

type Result struct {
	Checks []Check
	Ready  bool
}

// Session is a controlling terminal that has already entered raw mode.
// Implementations must make Restore idempotent.
type Session interface {
	Prompt(string) error
	ReadBounded(context.Context, time.Duration, int) ([]byte, error)
	Restore() error
	Close() error
}

type OpenFunc func() (Session, error)
type NotifyContextFunc func(context.Context) (context.Context, func())

type Probe struct {
	Open          OpenFunc
	NotifyContext NotifyContextFunc
	PerKeyTimeout time.Duration
	MaxInputBytes int
}

type step struct {
	id       string
	name     string
	display  string
	expected [][]byte
	guidance string
}

// Run checks rewrite, undo, Enter, and Ctrl+C in that order. Captured bytes
// exist only in the current comparison and are discarded immediately.
func (probe Probe) Run(ctx context.Context, rewriteValue, undoValue string) Result {
	rewrite, err := keychord.Parse(rewriteValue)
	if err != nil {
		return invalidConfigurationResult("rewrite_key")
	}
	undo, err := keychord.Parse(undoValue)
	if err != nil || rewrite == undo {
		return invalidConfigurationResult("undo_key")
	}

	notifyContext := probe.NotifyContext
	if notifyContext == nil {
		notifyContext = defaultNotifyContext
	}
	probeContext, stop := notifyContext(ctx)
	defer stop()
	if err := probeContext.Err(); err != nil {
		return unavailableResult("key probe was cancelled before opening the controlling terminal")
	}

	open := probe.Open
	if open == nil {
		open = OpenControllingTTY
	}
	session, err := open()
	if err != nil {
		return unavailableResult("controlling terminal is unavailable")
	}
	cleanupPending := true
	defer func() {
		// This is an emergency guard for an unexpected Session implementation
		// panic. Normal paths below report restoration as a stable check.
		if cleanupPending {
			_ = session.Restore()
			_ = session.Close()
		}
	}()

	timeout := probe.PerKeyTimeout
	if timeout <= 0 {
		timeout = DefaultPerKeyTimeout
	}
	maxBytes := probe.MaxInputBytes
	if maxBytes <= 0 || maxBytes > DefaultMaxInputBytes {
		maxBytes = DefaultMaxInputBytes
	}

	result := Result{Checks: []Check{{Status: StatusPass, ID: CheckTTY, Detail: "opened the controlling terminal in temporary raw mode"}}}
	steps := []step{
		{
			id: CheckRewrite, name: "rewrite", display: rewrite.Display(), expected: [][]byte{rewrite.TerminalSequence().Bytes()},
			guidance: "Remap with `intent-sh config set rewrite_key <allowed-chord>`, start a new shell, and retry.",
		},
		{
			id: CheckUndo, name: "undo", display: undo.Display(), expected: [][]byte{undo.TerminalSequence().Bytes()},
			guidance: "Remap with `intent-sh config set undo_key <allowed-chord>`, start a new shell, and retry.",
		},
		{
			id: CheckEnter, name: "Enter", display: "Enter", expected: [][]byte{{'\r'}, {'\n'}},
			guidance: "Review terminal, shell, or tmux Enter mappings and retry; intent-sh will not change them.",
		},
		{
			id: CheckCancel, name: "cancellation", display: "Ctrl+C", expected: [][]byte{{0x03}},
			guidance: "Review terminal, shell, or tmux Ctrl+C mappings and retry; intent-sh will not change them.",
		},
	}

	stopped := false
	for _, item := range steps {
		if stopped {
			result.Checks = append(result.Checks, Check{Status: StatusSkip, ID: item.id, Detail: "not checked after an earlier terminal read failure"})
			continue
		}
		if err := session.Prompt("intent-sh key probe: press " + item.display + " now\r\n"); err != nil {
			result.Checks = append(result.Checks, Check{Status: StatusFail, ID: item.id, Detail: "could not write the bounded key prompt", Guidance: "Check controlling-terminal access and retry."})
			stopped = true
			continue
		}
		received, readErr := session.ReadBounded(probeContext, timeout, maxBytes)
		if readErr != nil {
			wipeBytes(received)
			result.Checks = append(result.Checks, readFailureCheck(item, readErr))
			if errors.Is(readErr, context.Canceled) || errors.Is(readErr, io.EOF) || !errors.Is(readErr, ErrTimeout) && !errors.Is(readErr, ErrTooManyBytes) {
				stopped = true
			}
			continue
		}
		matched := false
		for _, expected := range item.expected {
			if bytes.Equal(received, expected) {
				matched = true
				break
			}
		}
		if matched {
			result.Checks = append(result.Checks, Check{Status: StatusPass, ID: item.id, Detail: item.display + " was delivered as expected"})
		} else {
			result.Checks = append(result.Checks, Check{
				Status: StatusFail, ID: item.id,
				Detail:   item.display + " was intercepted or transformed; received " + SymbolicBytes(received),
				Guidance: item.guidance,
			})
		}
		wipeBytes(received)
	}

	restoreErr := session.Restore()
	closeErr := session.Close()
	cleanupPending = false
	if restoreErr != nil {
		result.Checks = append(result.Checks, Check{Status: StatusFail, ID: CheckRestore, Detail: "terminal mode could not be confirmed restored", Guidance: "Run `reset` in this terminal if input or echo appears abnormal."})
	} else if closeErr != nil {
		result.Checks = append(result.Checks, Check{Status: StatusFail, ID: CheckRestore, Detail: "terminal mode was restored but the controlling terminal could not be closed cleanly", Guidance: "Retry the diagnostic in a new shell."})
	} else {
		result.Checks = append(result.Checks, Check{Status: StatusPass, ID: CheckRestore, Detail: "original terminal mode was restored"})
	}
	result.Ready = allPassed(result.Checks)
	return result
}

func readFailureCheck(item step, err error) Check {
	detail := item.display + " delivery failed"
	switch {
	case errors.Is(err, ErrTimeout):
		detail = item.display + " was not received before the deadline"
	case errors.Is(err, ErrTooManyBytes):
		detail = item.display + " produced more than the bounded byte limit"
	case errors.Is(err, context.Canceled):
		detail = "key probe was cancelled while waiting for " + item.display
	case errors.Is(err, io.EOF):
		detail = "controlling terminal reached EOF while waiting for " + item.display
	default:
		detail = "controlling terminal read failed while waiting for " + item.display
	}
	return Check{Status: StatusFail, ID: item.id, Detail: detail, Guidance: item.guidance}
}

func invalidConfigurationResult(field string) Result {
	result := Result{Checks: []Check{{Status: StatusFail, ID: CheckTTY, Detail: field + " is invalid; the key probe did not open a terminal", Guidance: "Correct the reported configuration field and retry."}}}
	appendSkipped(&result, "not checked because binding configuration is invalid")
	return result
}

func unavailableResult(detail string) Result {
	result := Result{Checks: []Check{{Status: StatusFail, ID: CheckTTY, Detail: detail, Guidance: "Run `intent-sh doctor --keys` directly from an interactive terminal; ordinary `intent-sh doctor` remains non-interactive."}}}
	appendSkipped(&result, "not checked without a controlling terminal")
	return result
}

func appendSkipped(result *Result, detail string) {
	for _, id := range []string{CheckRewrite, CheckUndo, CheckEnter, CheckCancel, CheckRestore} {
		result.Checks = append(result.Checks, Check{Status: StatusSkip, ID: id, Detail: detail})
	}
}

func allPassed(checks []Check) bool {
	if len(checks) != 6 {
		return false
	}
	for _, check := range checks {
		if check.Status != StatusPass {
			return false
		}
	}
	return true
}

// SymbolicBytes renders at most the fixed probe byte limit. Printable input is
// represented as hexadecimal, never copied as arbitrary terminal text.
func SymbolicBytes(values []byte) string {
	if len(values) == 0 {
		return "<none>"
	}
	limit := len(values)
	if limit > DefaultMaxInputBytes {
		limit = DefaultMaxInputBytes
	}
	parts := make([]string, 0, limit+1)
	for _, value := range values[:limit] {
		switch value {
		case 0x1b:
			parts = append(parts, "<ESC>")
		case '\r':
			parts = append(parts, "<CR>")
		case '\n':
			parts = append(parts, "<LF>")
		case 0x03:
			parts = append(parts, "<Ctrl+C>")
		default:
			parts = append(parts, fmt.Sprintf("0x%02X", value))
		}
	}
	if len(values) > limit {
		parts = append(parts, "<truncated>")
	}
	return strings.Join(parts, " ")
}

func defaultNotifyContext(ctx context.Context) (context.Context, func()) {
	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	return notifyCtx, stop
}
