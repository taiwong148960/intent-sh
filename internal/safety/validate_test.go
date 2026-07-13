package safety

import (
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

func TestParseAcceptsOneSimpleCommandOrPipeline(t *testing.T) {
	t.Parallel()
	tests := []string{
		`ls -la`,
		`find . -type f -exec du -h {} + | sort -rh | head -n 10`,
		`grep needle < input.txt | sort > output.txt`,
		`printf '%s' "$(date)"`,
		`cat <(printf '%s' hi)`,
		`printf '%s' a |& grep a`,
		`FOO=bar env printf '%s' "$FOO"`,
	}
	for _, command := range tests {
		command := command
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			parsed, err := Parse(command)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", command, err)
			}
			if len(parsed.stages) == 0 {
				t.Fatal("Parse() returned no executable stages")
			}
		})
	}
}

func TestParseRejectsBoundaryViolations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
	}{
		{"empty", ""},
		{"whitespace", "  \t"},
		{"too long", strings.Repeat("x", MaxCommandBytes+1)},
		{"nul", "printf x\x00"},
		{"newline", "ls\npwd"},
		{"carriage return", "ls\rpwd"},
		{"escape control", "printf \\x1b" + "\x1b"},
		{"backtick fence", "```sh ls```"},
		{"tilde fence", "~~~sh ls~~~"},
		{"leading whitespace", " ls"},
		{"trailing whitespace", "ls "},
		{"English chatter", "Here is the command: ls"},
		{"Chinese chatter", "运行以下命令：ls"},
		{"shell comment", "ls # this lists files"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(test.command)
			if err == nil {
				t.Fatalf("Parse(%q) unexpectedly succeeded", test.command)
			}
			if got := apperr.KindOf(err); got != apperr.KindSafety {
				t.Fatalf("kind = %q, want safety", got)
			}
			if len(apperr.Message(err)) > 200 {
				t.Fatalf("error was not bounded: %q", apperr.Message(err))
			}
		})
	}
}

func TestParseRejectsDisallowedStructures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
	}{
		{"semicolon list", `ls; pwd`},
		{"trailing semicolon", `ls;`},
		{"and list", `ls && pwd`},
		{"or list", `ls || pwd`},
		{"background", `sleep 1 &`},
		{"negation", `! false`},
		{"subshell", `(pwd)`},
		{"brace group", `{ pwd; }`},
		{"if", `if true; then pwd; fi`},
		{"loop", `for x in a; do echo "$x"; done`},
		{"function", `f() { pwd; }`},
		{"here string", `cat <<< value`},
		{"heredoc", "cat <<EOF\nvalue\nEOF"},
		{"assignment only", `FOO=bar`},
		{"redirection only", `> output.txt`},
		{"nested list", `echo "$(pwd; id)"`},
		{"nested conditional", `echo "$(true && id)"`},
		{"process substitution list", `cat <(pwd; id)`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(test.command)
			if err == nil {
				t.Fatalf("Parse(%q) unexpectedly succeeded", test.command)
			}
			if got := apperr.KindOf(err); got != apperr.KindSafety {
				t.Fatalf("kind = %q, want safety; err=%v", got, err)
			}
		})
	}
}
