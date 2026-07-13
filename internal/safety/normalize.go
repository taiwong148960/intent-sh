package safety

import (
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type argument struct {
	value  string
	static bool
}

type normalizedRedirection struct {
	op     syntax.RedirOperator
	target argument
}

type normalizedStage struct {
	name              string
	args              []argument
	redirs            []normalizedRedirection
	privileged        bool
	environmentChange bool
	explicitPath      bool
	opaqueCommand     bool
	dynamic           bool
	nested            bool
	position          int
}

var assignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func normalizeStages(parsed *ParsedCommand) []normalizedStage {
	stages := make([]normalizedStage, 0, len(parsed.stages))
	for _, raw := range parsed.stages {
		stages = append(stages, normalizeRawStage(raw))
	}
	return stages
}

func normalizeRawStage(raw rawStage) normalizedStage {
	stage := normalizedStage{
		nested:            raw.nested,
		position:          raw.position,
		environmentChange: len(raw.call.Assigns) > 0,
	}
	words := make([]argument, 0, len(raw.call.Args))
	for _, word := range raw.call.Args {
		arg := staticWord(word)
		words = append(words, arg)
		if !arg.static {
			stage.dynamic = true
		}
	}
	for _, assign := range raw.call.Assigns {
		if assign.Array != nil || assign.Index != nil {
			stage.dynamic = true
		}
		if assign.Value != nil && !staticWord(assign.Value).static {
			stage.dynamic = true
		}
	}
	for _, redirect := range raw.redirs {
		stage.redirs = append(stage.redirs, normalizedRedirection{op: redirect.Op, target: staticWord(redirect.Word)})
	}
	if len(words) == 0 || !words[0].static {
		stage.dynamic = true
		return stage
	}

	name := commandBase(words[0].value)
	args := words[1:]
	stage.explicitPath = executableHasPath(words[0].value)
	for {
		if name == "env" {
			stage.environmentChange = true
			stage.opaqueCommand = stage.opaqueCommand || hasEnvSplitString(args)
		}
		index, wrapper, privileged, uncertain := wrappedCommandIndex(name, args)
		stage.privileged = stage.privileged || privileged
		if uncertain {
			stage.dynamic = true
		}
		if !wrapper || index < 0 {
			break
		}
		if index >= len(args) || !args[index].static {
			stage.dynamic = true
			name = ""
			args = nil
			break
		}
		stage.explicitPath = stage.explicitPath || executableHasPath(args[index].value)
		name = commandBase(args[index].value)
		args = args[index+1:]
	}
	stage.name = name
	stage.args = args
	return stage
}

func wrappedCommandIndex(name string, args []argument) (index int, wrapper, privileged, uncertain bool) {
	switch name {
	case "sudo":
		index, uncertain = optionCommandIndex(args, sudoValueOptions, true)
		return index, index >= 0, true, uncertain
	case "env":
		index, uncertain = optionCommandIndex(args, envValueOptions, true)
		return index, index >= 0, false, uncertain
	case "command":
		for _, arg := range args {
			if !arg.static {
				return -1, false, false, true
			}
			if arg.value == "--" || arg.value == "-" || !strings.HasPrefix(arg.value, "-") {
				break
			}
			if !strings.HasPrefix(arg.value, "--") && strings.ContainsAny(strings.TrimPrefix(arg.value, "-"), "vV") {
				return -1, false, false, false
			}
		}
		index, uncertain = optionCommandIndex(args, nil, false)
		return index, index >= 0, false, uncertain
	case "builtin":
		if len(args) > 0 && args[0].static && args[0].value == "-p" {
			return -1, false, false, false
		}
		index, uncertain = optionCommandIndex(args, nil, false)
		return index, index >= 0, false, uncertain
	default:
		return -1, false, false, false
	}
}

var sudoValueOptions = map[string]bool{
	"-u": true, "--user": true, "-g": true, "--group": true,
	"-h": true, "--host": true, "-p": true, "--prompt": true,
	"-C": true, "--close-from": true, "-T": true, "--command-timeout": true,
	"-D": true, "--chdir": true, "-R": true, "--chroot": true,
	"-r": true, "--role": true, "-t": true, "--type": true,
}

var envValueOptions = map[string]bool{
	"-u": true, "--unset": true, "-C": true, "--chdir": true,
	"-S": true, "--split-string": true,
}

func optionCommandIndex(args []argument, valueOptions map[string]bool, skipAssignments bool) (int, bool) {
	uncertain := false
	for index := 0; index < len(args); index++ {
		if !args[index].static {
			return -1, true
		}
		value := args[index].value
		if value == "--" {
			if index+1 < len(args) {
				return index + 1, false
			}
			return -1, false
		}
		if skipAssignments && assignmentPattern.MatchString(value) {
			continue
		}
		if strings.HasPrefix(value, "-") && value != "-" {
			option := value
			if before, _, ok := strings.Cut(option, "="); ok {
				option = before
			}
			if valueOptions[option] && !strings.Contains(value, "=") {
				if index+1 >= len(args) {
					return -1, true
				}
				uncertain = uncertain || !args[index+1].static
				index++
			}
			continue
		}
		return index, uncertain
	}
	return -1, uncertain
}

func staticWord(word *syntax.Word) argument {
	if word == nil {
		return argument{}
	}
	var builder strings.Builder
	for _, part := range word.Parts {
		if !appendStaticPart(&builder, part, false) {
			return argument{static: false}
		}
	}
	return argument{value: builder.String(), static: true}
}

func appendStaticPart(builder *strings.Builder, part syntax.WordPart, doubleQuoted bool) bool {
	switch value := part.(type) {
	case *syntax.Lit:
		builder.WriteString(unescapeLiteral(value.Value, doubleQuoted))
		return true
	case *syntax.SglQuoted:
		if value.Dollar {
			return false
		}
		builder.WriteString(value.Value)
		return true
	case *syntax.DblQuoted:
		if value.Dollar {
			return false
		}
		for _, nested := range value.Parts {
			if !appendStaticPart(builder, nested, true) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func unescapeLiteral(value string, doubleQuoted bool) string {
	if !strings.ContainsRune(value, '\\') {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			builder.WriteByte(value[index])
			continue
		}
		next := value[index+1]
		if doubleQuoted && next != '$' && next != '`' && next != '"' && next != '\\' {
			builder.WriteByte(value[index])
			continue
		}
		builder.WriteByte(next)
		index++
	}
	return builder.String()
}

func commandBase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.Base(value)
}

func executableHasPath(value string) bool {
	return strings.ContainsRune(value, '/')
}

func hasEnvSplitString(args []argument) bool {
	for _, arg := range args {
		if !arg.static {
			continue
		}
		value := arg.value
		if value == "-S" || strings.HasPrefix(value, "-S") || value == "--split-string" || strings.HasPrefix(value, "--split-string=") {
			return true
		}
	}
	return false
}
