// Package apperr defines stable error categories and process exit codes.
package apperr

import (
	"errors"
	"fmt"

	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

const maxSafeMessageBytes = 2048

// Kind is a stable machine-readable error category.
type Kind string

const (
	KindInternal            Kind = "internal"
	KindInvalidInput        Kind = "invalid_input"
	KindConfiguration       Kind = "configuration"
	KindProviderUnavailable Kind = "provider_unavailable"
	KindTimeout             Kind = "timeout"
	KindCancelled           Kind = "cancelled"
	KindProviderOutput      Kind = "provider_output"
	KindSafety              Kind = "safety_rejection"
	KindProtocol            Kind = "protocol_incompatible"
)

const (
	ExitOK                  = 0
	ExitInternal            = 1
	ExitInvalidInput        = 2
	ExitConfiguration       = 3
	ExitProviderUnavailable = 4
	ExitTimeout             = 5
	ExitProviderOutput      = 6
	ExitSafety              = 7
	ExitProtocol            = 8
	ExitCancelled           = 130
)

// Error carries a safe user-facing message while retaining an internal cause.
type Error struct {
	Kind    Kind
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Op == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// New constructs an error whose message is safe to show to a terminal user.
func New(kind Kind, op, message string) error {
	return &Error{Kind: kind, Op: op, Message: textsafe.Terminal(message, maxSafeMessageBytes)}
}

// Wrap constructs an error with a retained internal cause.
func Wrap(kind Kind, op, message string, err error) error {
	return &Error{Kind: kind, Op: op, Message: textsafe.Terminal(message, maxSafeMessageBytes), Err: err}
}

// KindOf returns KindInternal for unclassified errors.
func KindOf(err error) Kind {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind
	}
	return KindInternal
}

// Message returns only the bounded, intentional user-facing text.
func Message(err error) string {
	if err == nil {
		return ""
	}
	var target *Error
	if errors.As(err, &target) && target.Message != "" {
		return textsafe.Terminal(target.Message, maxSafeMessageBytes)
	}
	return "intent-sh encountered an internal error"
}

// ExitCode maps stable error kinds to CLI exit codes.
func ExitCode(err error) int {
	switch KindOf(err) {
	case KindInvalidInput:
		return ExitInvalidInput
	case KindConfiguration:
		return ExitConfiguration
	case KindProviderUnavailable:
		return ExitProviderUnavailable
	case KindTimeout:
		return ExitTimeout
	case KindCancelled:
		return ExitCancelled
	case KindProviderOutput:
		return ExitProviderOutput
	case KindSafety:
		return ExitSafety
	case KindProtocol:
		return ExitProtocol
	default:
		return ExitInternal
	}
}
