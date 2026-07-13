package safety

import (
	"regexp"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

const (
	ReasonKnownReadOnly       = "known-read-only"
	ReasonUnknownCommand      = "unknown-command"
	ReasonDynamicOperation    = "dynamic-operation"
	ReasonEnvironmentChange   = "environment-change"
	ReasonExplicitPath        = "explicit-executable-path"
	ReasonPrivilegedOperation = "privileged-operation"
	ReasonStateChange         = "state-change"
	ReasonOutputRedirection   = "output-redirection"
	ReasonRecursiveDelete     = "recursive-delete"
	ReasonPrivilegedDelete    = "privileged-delete"
	ReasonDiskDestruction     = "disk-destruction"
	ReasonDownloadToShell     = "download-to-shell"
	ReasonDestructiveGitClean = "destructive-git-clean"
	ReasonHardGitReset        = "hard-git-reset"
	ReasonShutdown            = "system-shutdown"
	ReasonDestructiveSQL      = "destructive-sql"
	ReasonShellEvaluation     = "shell-evaluation"
	ReasonFindDelete          = "find-delete"
	ReasonProviderReviewHint  = "provider-review-hint"
	ReasonProviderDangerHint  = "provider-dangerous-hint"
)

var destructiveSQL = regexp.MustCompile(`(?i)(^|[;[:space:]])(drop[[:space:]]+(database|schema|table|index)|truncate([[:space:]]+table)?|delete[[:space:]]+from)([;[:space:]]|$)`)
var sqlBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

var readOnlyCommands = map[string]bool{
	"basename": true, "cat": true, "cut": true, "date": true, "df": true,
	"dirname": true, "du": true, "echo": true, "file": true, "grep": true,
	"head": true, "id": true, "jq": true, "lsof": true, "ls": true,
	"printf": true, "ps": true, "pwd": true, "readlink": true, "realpath": true,
	"rg": true, "sort": true, "stat": true, "tail": true, "tr": true,
	"true": true, "false": true, "uname": true, "uniq": true, "wc": true,
	"which": true, "whoami": true,
}

var stateChangingCommands = map[string]bool{
	"chmod": true, "chown": true, "chgrp": true, "cp": true, "install": true,
	"kill": true, "killall": true, "ln": true, "mkdir": true, "mv": true,
	"pkill": true, "rm": true, "rmdir": true, "sed": true, "tee": true,
	"touch": true,
}

// Classify derives risk solely from the validated AST.
func Classify(parsed *ParsedCommand) Decision {
	stages := normalizeStages(parsed)
	decision := safeDecision()
	if downloadToShell(stages) {
		decision = dangerousDecision(ReasonDownloadToShell, "downloaded content is piped directly to a shell")
	}
	for _, stage := range stages {
		decision = raiseDecision(decision, classifyNormalizedStage(stage))
		for _, redirect := range stage.redirs {
			decision = raiseDecision(decision, classifyRedirection(redirect))
		}
	}
	return decision
}

func classifyNormalizedStage(stage normalizedStage) Decision {
	decision := classifyStage(stage)
	if stage.opaqueCommand {
		decision = raiseDecision(decision, dangerousDecision(ReasonShellEvaluation, "env split-string executes an opaque command string"))
	}
	if stage.privileged {
		decision = raiseDecision(decision, reviewDecision(ReasonPrivilegedOperation, "privileged execution requires review"))
	}
	if stage.environmentChange {
		decision = raiseDecision(decision, reviewDecision(ReasonEnvironmentChange, "environment changes can alter command resolution or behavior"))
	}
	if stage.explicitPath {
		decision = raiseDecision(decision, reviewDecision(ReasonExplicitPath, "an explicit executable path may not refer to the expected program"))
	}
	return decision
}

func classifyStage(stage normalizedStage) Decision {
	if stage.name == "" {
		return reviewDecision(ReasonDynamicOperation, "command name is determined dynamically")
	}
	if stage.name == "mkfs" || strings.HasPrefix(stage.name, "mkfs.") {
		return dangerousDecision(ReasonDiskDestruction, stage.name+" creates a filesystem and can destroy data")
	}

	switch stage.name {
	case "rm":
		if stage.privileged {
			return dangerousDecision(ReasonPrivilegedDelete, "privileged deletion can remove protected data")
		}
		if hasOption(stage.args, "r", "recursive") || hasOption(stage.args, "R", "recursive") {
			return dangerousDecision(ReasonRecursiveDelete, "recursive deletion can remove an entire tree")
		}
		return reviewDecision(ReasonStateChange, "rm removes files or directories")
	case "dd", "fdisk", "sfdisk", "cfdisk", "gdisk", "sgdisk", "parted", "wipefs", "shred":
		return dangerousDecision(ReasonDiskDestruction, stage.name+" can irreversibly overwrite storage")
	case "newfs", "newfs_apfs", "newfs_hfs":
		return dangerousDecision(ReasonDiskDestruction, stage.name+" creates a filesystem and can destroy data")
	case "diskutil":
		if hasAnyStaticArg(stage.args, "eraseDisk", "eraseVolume", "partitionDisk", "zeroDisk", "secureErase") {
			return dangerousDecision(ReasonDiskDestruction, "diskutil operation can erase or repartition storage")
		}
		return reviewDecision(ReasonStateChange, "diskutil may modify disks or volumes")
	case "shutdown", "reboot", "poweroff", "halt":
		return dangerousDecision(ReasonShutdown, stage.name+" stops or restarts the system")
	case "systemctl", "loginctl":
		if hasAnyStaticArg(stage.args, "reboot", "poweroff", "halt", "kexec") {
			return dangerousDecision(ReasonShutdown, stage.name+" operation stops or restarts the system")
		}
		return reviewDecision(ReasonStateChange, stage.name+" may change system state")
	case "init", "telinit":
		if hasAnyStaticArg(stage.args, "0", "6") {
			return dangerousDecision(ReasonShutdown, stage.name+" runlevel stops or restarts the system")
		}
		return reviewDecision(ReasonStateChange, stage.name+" may change system state")
	case "eval", "source", ".":
		return dangerousDecision(ReasonShellEvaluation, stage.name+" executes shell text dynamically")
	case "bash", "sh", "zsh", "dash", "ksh", "fish":
		if hasExactArg(stage.args, "-c") || hasExactArg(stage.args, "--command") {
			return dangerousDecision(ReasonShellEvaluation, stage.name+" -c executes a nested shell program")
		}
		return reviewDecision(ReasonStateChange, "starting a nested shell requires review")
	case "git":
		return classifyGit(stage)
	case "docker", "podman":
		return classifyContainer(stage)
	case "find":
		return classifyFind(stage)
	case "xargs":
		return classifyXargs(stage)
	case "psql", "mysql", "mariadb", "sqlite3", "mongosh", "redis-cli":
		if isDestructiveDatabaseCommand(stage.name, stage.args) {
			return dangerousDecision(ReasonDestructiveSQL, "database command contains a destructive statement")
		}
		return reviewDecision(ReasonStateChange, "database client commands may change persistent data")
	case "command":
		if hasAnyStaticArg(stage.args, "-v", "-V") && !stage.dynamic {
			return safeDecision()
		}
		return reviewDecision(ReasonUnknownCommand, "command wrapper could not be resolved safely")
	}

	if stage.name == "sed" {
		if hasOption(stage.args, "i", "in-place") {
			return reviewDecision(ReasonStateChange, "sed in-place mode modifies files")
		}
		return reviewDecision(ReasonStateChange, "sed programs can modify files or execute extensions")
	}
	if stage.name == "printf" && hasExactArg(stage.args, "-v") {
		return reviewDecision(ReasonStateChange, "printf -v changes a shell variable")
	}
	if stage.name == "tee" && hasRawDeviceArg(stage.args) {
		return dangerousDecision(ReasonDiskDestruction, "tee can overwrite a raw storage device")
	}
	if stage.name == "cp" && hasRawDeviceArg(stage.args) {
		return dangerousDecision(ReasonDiskDestruction, "cp can overwrite a raw storage device")
	}
	if stage.dynamic {
		return reviewDecision(ReasonDynamicOperation, stage.name+" contains dynamically expanded arguments")
	}
	if readOnlyCommands[stage.name] {
		return safeDecision()
	}
	if stateChangingCommands[stage.name] {
		return reviewDecision(ReasonStateChange, stage.name+" can change local state")
	}
	return reviewDecision(ReasonUnknownCommand, "unknown command defaults to review")
}

func classifyGit(stage normalizedStage) Decision {
	index, uncertain := optionCommandIndex(stage.args, gitGlobalValueOptions, false)
	if index < 0 || !stage.args[index].static {
		return reviewDecision(ReasonDynamicOperation, "Git operation could not be determined statically")
	}
	subcommand, args := stage.args[index].value, stage.args[index+1:]
	switch subcommand {
	case "log", "status", "diff", "show", "grep", "ls-files", "rev-parse", "describe":
		if stage.dynamic || uncertain {
			return reviewDecision(ReasonDynamicOperation, "Git arguments contain dynamic values")
		}
		return safeDecision()
	case "clean":
		if hasOption(args, "n", "dry-run") {
			if stage.dynamic || uncertain {
				return reviewDecision(ReasonDynamicOperation, "Git clean preview contains dynamic values")
			}
			return safeDecision()
		}
		if hasOption(args, "f", "force") {
			return dangerousDecision(ReasonDestructiveGitClean, "git clean --force permanently removes untracked files")
		}
		return reviewDecision(ReasonStateChange, "git clean may remove untracked files")
	case "reset":
		if hasExactArg(args, "--hard") || hasExactArg(args, "--merge") || hasExactArg(args, "--keep") {
			return dangerousDecision(ReasonHardGitReset, "git reset mode can discard working-tree changes")
		}
		return reviewDecision(ReasonStateChange, "git reset changes repository state")
	case "branch":
		if hasOption(args, "d", "delete") || hasExactArg(args, "-D") || hasOption(args, "m", "move") || hasOption(args, "M", "move") {
			return reviewDecision(ReasonStateChange, "git branch operation changes references")
		}
		if stage.dynamic || uncertain {
			return reviewDecision(ReasonDynamicOperation, "Git branch arguments contain dynamic values")
		}
		return safeDecision()
	default:
		return reviewDecision(ReasonStateChange, "Git subcommand may change repository state")
	}
}

var gitGlobalValueOptions = map[string]bool{
	"-C": true, "-c": true, "--exec-path": true, "--git-dir": true,
	"--work-tree": true, "--namespace": true, "--super-prefix": true,
	"--config-env": true,
}

func classifyContainer(stage normalizedStage) Decision {
	index, uncertain := optionCommandIndex(stage.args, containerGlobalValueOptions, false)
	if index < 0 || uncertain || stage.dynamic || !stage.args[index].static {
		return reviewDecision(ReasonDynamicOperation, "container operation could not be determined statically")
	}
	subcommand := stage.args[index].value
	switch subcommand {
	case "ps", "images", "inspect", "logs", "stats", "version", "info":
		return safeDecision()
	default:
		return reviewDecision(ReasonStateChange, stage.name+" "+subcommand+" may change container state")
	}
}

var containerGlobalValueOptions = map[string]bool{
	"--config": true, "-c": true, "--context": true, "-H": true,
	"--host": true, "-l": true, "--log-level": true,
}

func classifyFind(stage normalizedStage) Decision {
	decision := safeDecision()
	dynamic := stage.dynamic
	for index := 0; index < len(stage.args); index++ {
		arg := stage.args[index]
		if !arg.static {
			dynamic = true
			continue
		}
		switch arg.value {
		case "-delete":
			return dangerousDecision(ReasonFindDelete, "find -delete can recursively remove matched paths")
		case "-ok", "-okdir":
			decision = raiseDecision(decision, reviewDecision(ReasonStateChange, "find action may execute a state-changing command"))
		case "-exec", "-execdir":
			end := index + 1
			for end < len(stage.args) && stage.args[end].static && stage.args[end].value != ";" && stage.args[end].value != "+" {
				end++
			}
			if end == index+1 || end >= len(stage.args) {
				return reviewDecision(ReasonDynamicOperation, "find -exec command could not be resolved")
			}
			embedded := normalizeArguments(stage.args[index+1 : end])
			decision = raiseDecision(decision, classifyNormalizedStage(embedded))
			index = end
		}
	}
	if dynamic {
		return raiseDecision(decision, reviewDecision(ReasonDynamicOperation, "find expression contains dynamic values"))
	}
	return decision
}

func classifyXargs(stage normalizedStage) Decision {
	index, uncertain := optionCommandIndex(stage.args, xargsValueOptions, false)
	if uncertain || index < 0 {
		return reviewDecision(ReasonDynamicOperation, "xargs command could not be resolved statically")
	}
	embedded := normalizeArguments(stage.args[index:])
	decision := classifyNormalizedStage(embedded)
	if decision.Level == LevelDangerous {
		return decision
	}
	return reviewDecision(ReasonStateChange, "xargs applies a command to input-derived arguments")
}

var xargsValueOptions = map[string]bool{
	"-a": true, "--arg-file": true, "-d": true, "--delimiter": true,
	"-E": true, "--eof": true, "-I": true, "--replace": true,
	"-L": true, "--max-lines": true, "-n": true, "--max-args": true,
	"-P": true, "--max-procs": true, "-s": true, "--max-chars": true,
}

func normalizeArguments(words []argument) normalizedStage {
	if len(words) == 0 || !words[0].static {
		return normalizedStage{dynamic: true}
	}
	stage := normalizedStage{
		name:         commandBase(words[0].value),
		args:         words[1:],
		explicitPath: executableHasPath(words[0].value),
	}
	for _, arg := range words {
		if !arg.static {
			stage.dynamic = true
		}
	}
	for {
		if stage.name == "env" {
			stage.environmentChange = true
			stage.opaqueCommand = stage.opaqueCommand || hasEnvSplitString(stage.args)
		}
		index, wrapper, privileged, uncertain := wrappedCommandIndex(stage.name, stage.args)
		stage.privileged = stage.privileged || privileged
		stage.dynamic = stage.dynamic || uncertain
		if !wrapper || index < 0 {
			break
		}
		if index >= len(stage.args) || !stage.args[index].static {
			stage.name = ""
			stage.args = nil
			stage.dynamic = true
			break
		}
		stage.explicitPath = stage.explicitPath || executableHasPath(stage.args[index].value)
		stage.name = commandBase(stage.args[index].value)
		stage.args = stage.args[index+1:]
	}
	return stage
}

func classifyRedirection(redirect normalizedRedirection) Decision {
	if redirectionWrites(redirect.op) && redirect.target.static && isRawDevicePath(redirect.target.value) {
		return dangerousDecision(ReasonDiskDestruction, "redirection can overwrite a raw storage device")
	}
	switch redirect.op {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll:
		if redirect.target.static && redirect.target.value == "/dev/null" {
			return safeDecision()
		}
		return reviewDecision(ReasonOutputRedirection, "output redirection can create or overwrite a file")
	case syntax.DplOut:
		if redirect.target.static && (redirect.target.value == "-" || isFileDescriptor(redirect.target.value)) {
			return safeDecision()
		}
		return reviewDecision(ReasonOutputRedirection, "output duplication may write to a file")
	default:
		return safeDecision()
	}
}

func redirectionWrites(operator syntax.RedirOperator) bool {
	switch operator {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut, syntax.DplOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll:
		return true
	default:
		return false
	}
}

func downloadToShell(stages []normalizedStage) bool {
	seenDownload := false
	topShell := false
	nestedDownload := false
	nestedShell := false
	for _, stage := range stages {
		if stage.nested {
			if stage.name == "curl" || stage.name == "wget" {
				nestedDownload = true
			}
			nestedShell = nestedShell || isShell(stage.name)
			continue
		}
		if stage.name == "curl" || stage.name == "wget" {
			seenDownload = true
			continue
		}
		if seenDownload && isShell(stage.name) {
			return true
		}
		topShell = topShell || isShell(stage.name)
	}
	return topShell && nestedDownload || seenDownload && nestedShell
}

func isDestructiveDatabaseCommand(name string, args []argument) bool {
	text := sqlBlockComment.ReplaceAllString(joinStaticArgs(args), " ")
	lower := strings.ToLower(text)
	if destructiveSQL.MatchString(lower) {
		return true
	}
	if name == "mongosh" && (strings.Contains(lower, ".dropdatabase(") || strings.Contains(lower, ".drop(")) {
		return true
	}
	if name == "redis-cli" && (containsWord(lower, "flushall") || containsWord(lower, "flushdb")) {
		return true
	}
	return false
}

func containsWord(text, word string) bool {
	for _, field := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		if field == word {
			return true
		}
	}
	return false
}

func hasRawDeviceArg(args []argument) bool {
	for _, arg := range args {
		if arg.static && isRawDevicePath(arg.value) {
			return true
		}
	}
	return false
}

func isRawDevicePath(value string) bool {
	for _, prefix := range []string{"/dev/sd", "/dev/hd", "/dev/vd", "/dev/xvd", "/dev/nvme", "/dev/disk", "/dev/rdisk", "/dev/mmcblk"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func isShell(name string) bool {
	switch name {
	case "bash", "sh", "zsh", "dash", "ksh", "fish":
		return true
	default:
		return false
	}
}

func hasOption(args []argument, short, long string) bool {
	for _, arg := range args {
		if !arg.static {
			continue
		}
		value := arg.value
		if value == "--" {
			return false
		}
		if value == "--"+long || strings.HasPrefix(value, "--"+long+"=") {
			return true
		}
		if strings.HasPrefix(value, "-") && !strings.HasPrefix(value, "--") && strings.Contains(strings.TrimPrefix(value, "-"), short) {
			return true
		}
	}
	return false
}

func hasExactArg(args []argument, want string) bool {
	for _, arg := range args {
		if arg.static && arg.value == want {
			return true
		}
	}
	return false
}

func hasAnyStaticArg(args []argument, wants ...string) bool {
	for _, want := range wants {
		if hasExactArg(args, want) {
			return true
		}
	}
	return false
}

func firstOperand(args []argument) (string, []argument) {
	for index, arg := range args {
		if !arg.static {
			return "", nil
		}
		if arg.value == "--" {
			if index+1 < len(args) && args[index+1].static {
				return args[index+1].value, args[index+2:]
			}
			return "", nil
		}
		if strings.HasPrefix(arg.value, "-") {
			continue
		}
		return arg.value, args[index+1:]
	}
	return "", nil
}

func joinStaticArgs(args []argument) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.static {
			parts = append(parts, arg.value)
		}
	}
	return strings.Join(parts, " ")
}

func isFileDescriptor(value string) bool {
	_, err := strconv.ParseUint(value, 10, 31)
	return err == nil
}

func combineProviderHint(local Decision, hint string) Decision {
	switch Level(hint) {
	case LevelReview:
		return raiseDecision(local, reviewDecision(ReasonProviderReviewHint, "provider requested additional review"))
	case LevelDangerous:
		return raiseDecision(local, dangerousDecision(ReasonProviderDangerHint, "provider flagged the command as dangerous"))
	default:
		return local
	}
}

func raiseDecision(current, candidate Decision) Decision {
	if levelRank(candidate.Level) > levelRank(current.Level) {
		return candidate
	}
	return current
}

func levelRank(level Level) int {
	switch level {
	case LevelDangerous:
		return 2
	case LevelReview:
		return 1
	default:
		return 0
	}
}

func safeDecision() Decision {
	return Decision{Level: LevelSafe, ReasonCode: ReasonKnownReadOnly, Reason: "no known risky pattern matched"}
}

func reviewDecision(code, reason string) Decision {
	return Decision{Level: LevelReview, ReasonCode: code, Reason: reason}
}

func dangerousDecision(code, reason string) Decision {
	return Decision{Level: LevelDangerous, ReasonCode: code, Reason: reason}
}
