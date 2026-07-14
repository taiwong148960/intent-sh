package keyprobe

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeRead struct {
	data []byte
	err  error
}

type fakeSession struct {
	reads        []fakeRead
	readIndex    int
	prompts      []string
	promptErr    error
	restoreErr   error
	closeErr     error
	restoreCalls int
	closeCalls   int
	beforeRead   func()
	returned     [][]byte
}

func (session *fakeSession) Prompt(value string) error {
	session.prompts = append(session.prompts, value)
	return session.promptErr
}

func (session *fakeSession) ReadBounded(ctx context.Context, _ time.Duration, _ int) ([]byte, error) {
	if session.beforeRead != nil {
		hook := session.beforeRead
		session.beforeRead = nil
		hook()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if session.readIndex >= len(session.reads) {
		return nil, io.EOF
	}
	item := session.reads[session.readIndex]
	session.readIndex++
	result := append([]byte(nil), item.data...)
	session.returned = append(session.returned, result)
	return result, item.err
}

func (session *fakeSession) Restore() error {
	session.restoreCalls++
	return session.restoreErr
}

func (session *fakeSession) Close() error {
	session.closeCalls++
	return session.closeErr
}

func TestProbeFakeTerminalCasesAlwaysRestore(t *testing.T) {
	t.Parallel()
	readFailure := errors.New("SECRET_LOW_LEVEL_READ_ERROR")
	tests := []struct {
		name        string
		reads       []fakeRead
		promptErr   error
		restoreErr  error
		wantStatus  map[string]Status
		wantDetail  string
		wantPrompts int
	}{
		{
			name:        "success",
			reads:       []fakeRead{{data: []byte{0x1b, 'g'}}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\r'}}, {data: []byte{0x03}}},
			wantStatus:  map[string]Status{CheckTTY: StatusPass, CheckRewrite: StatusPass, CheckUndo: StatusPass, CheckEnter: StatusPass, CheckCancel: StatusPass, CheckRestore: StatusPass},
			wantPrompts: 4,
		},
		{
			name:       "transformed input",
			reads:      []fakeRead{{data: []byte{0x1b, 'x'}}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\n'}}, {data: []byte{0x03}}},
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusPass, CheckEnter: StatusPass, CheckCancel: StatusPass, CheckRestore: StatusPass},
			wantDetail: "<ESC> 0x78", wantPrompts: 4,
		},
		{
			name:       "excessive bytes",
			reads:      []fakeRead{{err: ErrTooManyBytes}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\r'}}, {data: []byte{0x03}}},
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusPass, CheckRestore: StatusPass},
			wantDetail: "bounded byte limit", wantPrompts: 4,
		},
		{
			name:       "timeout",
			reads:      []fakeRead{{err: ErrTimeout}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\r'}}, {data: []byte{0x03}}},
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusPass, CheckRestore: StatusPass},
			wantDetail: "deadline", wantPrompts: 4,
		},
		{
			name:       "read failure",
			reads:      []fakeRead{{err: readFailure}},
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusSkip, CheckEnter: StatusSkip, CheckCancel: StatusSkip, CheckRestore: StatusPass},
			wantDetail: "read failed", wantPrompts: 1,
		},
		{
			name:       "EOF",
			reads:      []fakeRead{{err: io.EOF}},
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusSkip, CheckRestore: StatusPass},
			wantDetail: "reached EOF", wantPrompts: 1,
		},
		{
			name:       "prompt failure",
			promptErr:  errors.New("SECRET_PROMPT_WRITE_ERROR"),
			wantStatus: map[string]Status{CheckRewrite: StatusFail, CheckUndo: StatusSkip, CheckRestore: StatusPass},
			wantDetail: "could not write", wantPrompts: 1,
		},
		{
			name:       "restore failure",
			reads:      []fakeRead{{data: []byte{0x1b, 'g'}}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\r'}}, {data: []byte{0x03}}},
			restoreErr: errors.New("SECRET_RESTORE_ERROR"),
			wantStatus: map[string]Status{CheckRewrite: StatusPass, CheckUndo: StatusPass, CheckEnter: StatusPass, CheckCancel: StatusPass, CheckRestore: StatusFail},
			wantDetail: "could not be confirmed restored", wantPrompts: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := &fakeSession{reads: test.reads, promptErr: test.promptErr, restoreErr: test.restoreErr}
			result := (Probe{Open: func() (Session, error) { return session, nil }, PerKeyTimeout: time.Millisecond}).Run(context.Background(), "alt+g", "alt+u")
			if len(result.Checks) != 6 {
				t.Fatalf("check count = %d: %#v", len(result.Checks), result.Checks)
			}
			checks := checksByID(result)
			for id, want := range test.wantStatus {
				if checks[id].Status != want {
					t.Fatalf("%s status = %s, want %s; result=%#v", id, checks[id].Status, want, result)
				}
			}
			if test.wantDetail != "" && !strings.Contains(renderChecks(result), test.wantDetail) {
				t.Fatalf("result omitted %q: %#v", test.wantDetail, result)
			}
			if session.restoreCalls != 1 || session.closeCalls != 1 {
				t.Fatalf("restore/close calls = %d/%d", session.restoreCalls, session.closeCalls)
			}
			if len(session.prompts) != test.wantPrompts {
				t.Fatalf("prompt count = %d, want %d", len(session.prompts), test.wantPrompts)
			}
			for _, secret := range []string{"SECRET_LOW_LEVEL_READ_ERROR", "SECRET_PROMPT_WRITE_ERROR", "SECRET_RESTORE_ERROR"} {
				if strings.Contains(renderChecks(result), secret) {
					t.Fatalf("internal cause leaked: %#v", result)
				}
			}
		})
	}
}

func TestProbeContextCancellationAndSignalContextRestore(t *testing.T) {
	t.Parallel()
	probeContext, cancel := context.WithCancel(context.Background())
	session := &fakeSession{}
	session.beforeRead = cancel
	probe := Probe{
		Open: func() (Session, error) { return session, nil },
		NotifyContext: func(context.Context) (context.Context, func()) {
			return probeContext, func() {}
		},
	}
	result := probe.Run(context.Background(), "alt+g", "alt+u")
	checks := checksByID(result)
	if checks[CheckRewrite].Status != StatusFail || !strings.Contains(checks[CheckRewrite].Detail, "cancelled") {
		t.Fatalf("cancellation result = %#v", result)
	}
	if checks[CheckUndo].Status != StatusSkip || checks[CheckRestore].Status != StatusPass || session.restoreCalls != 1 {
		t.Fatalf("cancelled probe did not stop and restore: %#v, restores=%d", result, session.restoreCalls)
	}
}

func TestProbeEmergencyGuardRestoresAfterUnexpectedSessionPanic(t *testing.T) {
	t.Parallel()
	session := &fakeSession{beforeRead: func() { panic("test session panic") }}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("probe did not propagate the unexpected panic")
			}
		}()
		_ = (Probe{Open: func() (Session, error) { return session, nil }}).Run(context.Background(), "alt+g", "alt+u")
	}()
	if session.restoreCalls != 1 || session.closeCalls != 1 {
		t.Fatalf("emergency restore/close calls = %d/%d, want 1/1", session.restoreCalls, session.closeCalls)
	}
}

func TestProbeUnavailableAndInvalidConfigNeverOpenOrConsumeStdin(t *testing.T) {
	t.Parallel()
	opened := 0
	probe := Probe{Open: func() (Session, error) {
		opened++
		return nil, errors.New("no /dev/tty")
	}}
	result := probe.Run(context.Background(), "alt+g", "alt+u")
	if result.Ready || checksByID(result)[CheckTTY].Status != StatusFail || opened != 1 {
		t.Fatalf("unavailable result = %#v, opens=%d", result, opened)
	}
	result = probe.Run(context.Background(), "ctrl+c", "alt+u")
	if checksByID(result)[CheckTTY].Status != StatusFail || opened != 1 {
		t.Fatalf("invalid config opened terminal: %#v, opens=%d", result, opened)
	}
	for _, check := range result.Checks[1:] {
		if check.Status != StatusSkip {
			t.Fatalf("invalid config check was not skipped: %#v", check)
		}
	}
}

func TestProbePrivacyNeverRendersCapturedTextOrControls(t *testing.T) {
	t.Parallel()
	secret := "SECRET_PROMPT_CREDENTIAL_HISTORY"
	session := &fakeSession{reads: []fakeRead{
		{data: []byte(secret + "\x1b[31m")},
		{data: []byte{0x1b, 'u'}},
		{data: []byte{'\r'}},
		{data: []byte{0x03}},
	}}
	result := (Probe{Open: func() (Session, error) { return session, nil }}).Run(context.Background(), "alt+g", "alt+u")
	output := renderChecks(result) + strings.Join(session.prompts, "")
	if strings.Contains(output, secret) || strings.Contains(output, "\x1b[31m") || strings.Contains(output, "[31m") {
		t.Fatalf("captured text or raw control sequence leaked: %q", output)
	}
	if !strings.Contains(output, "0x53") || !strings.Contains(output, "<truncated>") {
		t.Fatalf("bounded symbolic mismatch detail missing: %q", output)
	}
	for _, prohibited := range []string{"provider", "token", "history contents", "shell buffer"} {
		if strings.Contains(strings.ToLower(output), prohibited) {
			t.Fatalf("probe unexpectedly rendered unrelated diagnostic content %q: %q", prohibited, output)
		}
	}
}

func TestProbeWipesReceivedBytesAfterComparisonAndReadFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		reads []fakeRead
	}{
		{
			name: "successful comparisons",
			reads: []fakeRead{
				{data: []byte{0x1b, 'g'}}, {data: []byte{0x1b, 'u'}}, {data: []byte{'\r'}}, {data: []byte{0x03}},
			},
		},
		{name: "partial read failure", reads: []fakeRead{{data: []byte("SECRET"), err: errors.New("read failed")}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := &fakeSession{reads: test.reads}
			_ = (Probe{Open: func() (Session, error) { return session, nil }}).Run(context.Background(), "alt+g", "alt+u")
			for readIndex, received := range session.returned {
				for byteIndex, value := range received {
					if value != 0 {
						t.Fatalf("received[%d][%d] was not wiped: %#v", readIndex, byteIndex, received)
					}
				}
			}
		})
	}
}

func TestSymbolicBytesIsBoundedAndNeverPrintsArbitraryText(t *testing.T) {
	t.Parallel()
	input := append([]byte{0x1b, '\r', '\n', 0x03}, []byte("SECRET-CREDENTIAL")...)
	got := SymbolicBytes(input)
	for _, want := range []string{"<ESC>", "<CR>", "<LF>", "<Ctrl+C>", "0x53", "<truncated>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SymbolicBytes() = %q, omitted %q", got, want)
		}
	}
	if strings.Contains(got, "SECRET") || strings.ContainsAny(got, "\x1b\r\n") {
		t.Fatalf("SymbolicBytes() leaked raw input: %q", got)
	}
}

func checksByID(result Result) map[string]Check {
	checks := make(map[string]Check, len(result.Checks))
	for _, check := range result.Checks {
		checks[check.ID] = check
	}
	return checks
}

func renderChecks(result Result) string {
	var builder strings.Builder
	for _, check := range result.Checks {
		builder.WriteString(string(check.Status))
		builder.WriteByte(' ')
		builder.WriteString(check.ID)
		builder.WriteByte(' ')
		builder.WriteString(check.Detail)
		builder.WriteByte(' ')
		builder.WriteString(check.Guidance)
		builder.WriteByte('\n')
	}
	return builder.String()
}
