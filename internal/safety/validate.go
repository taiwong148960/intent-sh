package safety

import (
	"fmt"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"mvdan.cc/sh/v3/syntax"
)

// Parse enforces the command boundary and one-command AST policy.
func Parse(command string) (*ParsedCommand, error) {
	if err := validateBoundary(command); err != nil {
		return nil, err
	}
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash), syntax.KeepComments(true))
	file, err := parser.Parse(strings.NewReader(command), "provider-command")
	if err != nil {
		return nil, safetyWrap("parse command", "generated command is not valid portable shell syntax", err)
	}
	if hasComment(file) {
		return nil, safetyError("parse command", "generated command contained surrounding commentary")
	}
	parsed := &ParsedCommand{Source: command, File: file}
	if err := validateStatementList(file.Stmts, parsed, false); err != nil {
		return nil, err
	}
	if err := validateNestedPrograms(file, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateBoundary(command string) error {
	if command == "" || strings.TrimSpace(command) == "" {
		return safetyError("validate command", "provider returned an empty command")
	}
	if len(command) > MaxCommandBytes {
		return safetyError("validate command", "generated command exceeded the 8 KiB limit")
	}
	if strings.ContainsRune(command, '\x00') {
		return safetyError("validate command", "generated command contained a NUL byte")
	}
	if strings.ContainsAny(command, "\r\n") {
		return safetyError("validate command", "generated command must be one line")
	}
	for _, r := range command {
		if r != '\t' && (r < 0x20 || r >= 0x7f && r <= 0x9f) {
			return safetyError("validate command", "generated command contained a control character")
		}
	}
	if command != strings.TrimSpace(command) {
		return safetyError("validate command", "generated command contained surrounding whitespace")
	}
	if strings.Contains(command, "```") || strings.Contains(command, "~~~") {
		return safetyError("validate command", "generated command contained a Markdown fence")
	}
	if looksLikeChatter(command) {
		return safetyError("validate command", "generated command contained surrounding commentary")
	}
	return nil
}

func looksLikeChatter(command string) bool {
	lower := strings.ToLower(command)
	prefixes := []string{
		"here is the command", "here's the command", "the command is", "command:",
		"run this command", "you can run", "use this command",
		"命令是", "运行以下命令", "使用以下命令", "可以运行",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func validateStatementList(stmts []*syntax.Stmt, parsed *ParsedCommand, nested bool) error {
	if len(stmts) != 1 {
		return safetyError("validate structure", "generated command must contain exactly one statement")
	}
	stmt := stmts[0]
	if stmt.Semicolon.IsValid() {
		return safetyError("validate structure", "generated command may not end with a statement separator")
	}
	return validateStatement(stmt, parsed, nested)
}

func validateStatement(stmt *syntax.Stmt, parsed *ParsedCommand, nested bool) error {
	if stmt == nil || stmt.Cmd == nil {
		return safetyError("validate structure", "generated command omitted an executable command")
	}
	if stmt.Background || stmt.Coprocess || stmt.Negated {
		return safetyError("validate structure", "background, coprocess, and negated commands are not allowed")
	}
	if err := validateRedirections(stmt.Redirs); err != nil {
		return err
	}
	switch command := stmt.Cmd.(type) {
	case *syntax.CallExpr:
		if len(command.Args) == 0 {
			return safetyError("validate structure", "assignment-only commands are not allowed")
		}
		parsed.stages = append(parsed.stages, rawStage{
			call:     command,
			redirs:   append([]*syntax.Redirect(nil), stmt.Redirs...),
			nested:   nested,
			position: len(parsed.stages),
		})
		return nil
	case *syntax.BinaryCmd:
		if command.Op != syntax.Pipe && command.Op != syntax.PipeAll {
			return safetyError("validate structure", "command lists and conditional chains are not allowed")
		}
		if len(stmt.Redirs) != 0 {
			return safetyError("validate structure", "redirection of an entire pipeline is not allowed")
		}
		if err := validatePipelineSide(command.X, parsed, nested); err != nil {
			return err
		}
		return validatePipelineSide(command.Y, parsed, nested)
	default:
		return safetyError("validate structure", fmt.Sprintf("compound shell construct %T is not allowed", stmt.Cmd))
	}
}

func validatePipelineSide(stmt *syntax.Stmt, parsed *ParsedCommand, nested bool) error {
	if stmt == nil {
		return safetyError("validate structure", "pipeline contained an empty stage")
	}
	if stmt.Background || stmt.Coprocess || stmt.Negated {
		return safetyError("validate structure", "pipeline contained a disallowed statement modifier")
	}
	// A valid |& token may populate Semicolon on an internal statement, so the
	// containing BinaryCmd operator—not this position field—is authoritative.
	return validateStatement(stmt, parsed, nested)
}

func validateRedirections(redirs []*syntax.Redirect) error {
	for _, redirect := range redirs {
		if redirect == nil {
			continue
		}
		if redirect.Hdoc != nil || redirect.Op == syntax.Hdoc || redirect.Op == syntax.DashHdoc || redirect.Op == syntax.WordHdoc {
			return safetyError("validate structure", "heredoc and here-string redirections are not allowed")
		}
	}
	return nil
}

func validateNestedPrograms(root syntax.Node, parsed *ParsedCommand) error {
	var validationErr error
	syntax.Walk(root, func(node syntax.Node) bool {
		if validationErr != nil || node == nil {
			return false
		}
		switch item := node.(type) {
		case *syntax.CmdSubst:
			validationErr = validateStatementList(item.Stmts, parsed, true)
			if validationErr == nil {
				validationErr = validateNestedList(item.Stmts, parsed)
			}
			return false
		case *syntax.ProcSubst:
			validationErr = validateStatementList(item.Stmts, parsed, true)
			if validationErr == nil {
				validationErr = validateNestedList(item.Stmts, parsed)
			}
			return false
		}
		return true
	})
	return validationErr
}

func validateNestedList(stmts []*syntax.Stmt, parsed *ParsedCommand) error {
	for _, stmt := range stmts {
		if err := validateNestedPrograms(stmt, parsed); err != nil {
			return err
		}
	}
	return nil
}

func hasComment(file *syntax.File) bool {
	found := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if _, ok := node.(*syntax.Comment); ok {
			found = true
			return false
		}
		return !found
	})
	return found
}

func safetyError(op, message string) error {
	return apperr.New(apperr.KindSafety, op, message)
}

func safetyWrap(op, message string, err error) error {
	return apperr.Wrap(apperr.KindSafety, op, message, err)
}
