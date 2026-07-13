// Package safety validates generated shell commands and derives local risk.
package safety

import (
	"context"

	"mvdan.cc/sh/v3/syntax"
)

const MaxCommandBytes = 8 * 1024

const (
	ShellBash = "bash"
	ShellZsh  = "zsh"
)

type Level string

const (
	LevelSafe      Level = "safe"
	LevelReview    Level = "review"
	LevelDangerous Level = "dangerous"
)

// Decision is the authoritative risk attached to a validated command.
type Decision struct {
	Command    string
	Level      Level
	ReasonCode string
	Reason     string
}

// ParsedCommand is an AST-validated command ready for syntax and risk checks.
type ParsedCommand struct {
	Source string
	File   *syntax.File
	stages []rawStage
}

type rawStage struct {
	call     *syntax.CallExpr
	redirs   []*syntax.Redirect
	nested   bool
	position int
}

// SyntaxChecker asks the declared shell to parse without executing.
type SyntaxChecker interface {
	Check(context.Context, string, string) error
}

// Engine orders boundary, AST, target-shell, and risk checks.
type Engine struct {
	Checker SyntaxChecker
}

func (e Engine) Evaluate(ctx context.Context, command, shell, providerHint string) (Decision, error) {
	parsed, err := Parse(command)
	if err != nil {
		return Decision{}, err
	}
	checker := e.Checker
	if checker == nil {
		checker = ExecSyntaxChecker{}
	}
	if err := checker.Check(ctx, shell, command); err != nil {
		return Decision{}, err
	}
	decision := Classify(parsed)
	decision = combineProviderHint(decision, providerHint)
	decision.Command = command
	return decision, nil
}
