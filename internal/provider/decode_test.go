package provider

import (
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/protocol"
)

func TestDecodeResultValidBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		status string
	}{
		{
			name:   "ok",
			input:  `{"status":"ok","command":"printf '%s\\n' hi","explanation":"prints","assumptions":[],"riskHint":"safe"}`,
			status: protocol.ProviderStatusOK,
		},
		{
			name:   "clarify",
			input:  `{"status":"clarify","question":"Which directory?"}`,
			status: protocol.ProviderStatusClarify,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeResult([]byte(test.input))
			if err != nil {
				t.Fatalf("DecodeResult() error = %v", err)
			}
			if got.Status != test.status {
				t.Fatalf("status = %q, want %q", got.Status, test.status)
			}
		})
	}
}

func TestDecodeResultRejectsInvalidShapeAndContent(t *testing.T) {
	t.Parallel()
	longCommand := strings.Repeat("x", 8193)
	longQuestion := strings.Repeat("q", 1025)
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"invalid UTF-8", string([]byte{'{', '"', 0xff, '"', ':', '1', '}'})},
		{"unknown field", `{"status":"clarify","question":"where?","extra":true}`},
		{"missing status", `{"question":"where?"}`},
		{"unknown status", `{"status":"maybe","question":"where?"}`},
		{"duplicate status", `{"status":"clarify","status":"ok","question":"where?"}`},
		{"duplicate branch field", `{"status":"ok","command":"pwd","command":"rm -rf /","explanation":"x","assumptions":[],"riskHint":"safe"}`},
		{"ok missing command", `{"status":"ok","explanation":"x","assumptions":[],"riskHint":"safe"}`},
		{"ok cross branch", `{"status":"ok","command":"pwd","explanation":"x","assumptions":[],"riskHint":"safe","question":"why?"}`},
		{"clarify cross branch", `{"status":"clarify","question":"where?","command":"pwd"}`},
		{"empty question", `{"status":"clarify","question":""}`},
		{"invalid hint", `{"status":"ok","command":"pwd","explanation":"x","assumptions":[],"riskHint":"certain"}`},
		{"multiple values", `{"status":"clarify","question":"one?"} {"status":"clarify","question":"two?"}`},
		{"trailing chatter", `{"status":"clarify","question":"one?"} chatter`},
		{"long command", `{"status":"ok","command":"` + longCommand + `","explanation":"x","assumptions":[],"riskHint":"safe"}`},
		{"long question", `{"status":"clarify","question":"` + longQuestion + `"}`},
		{"too many assumptions", `{"status":"ok","command":"pwd","explanation":"x","assumptions":["1","2","3","4","5","6","7","8","9","10","11","12","13","14","15","16","17"],"riskHint":"safe"}`},
		{"non object", `[]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeResult([]byte(test.input))
			if err == nil {
				t.Fatal("DecodeResult() unexpectedly succeeded")
			}
			if got := apperr.KindOf(err); got != apperr.KindProviderOutput {
				t.Fatalf("kind = %q, want %q", got, apperr.KindProviderOutput)
			}
		})
	}
}

func TestDecodeResultEnforcesWholeOutputBound(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult([]byte(strings.Repeat(" ", protocol.MaxProviderOutputBytes+1)))
	if apperr.KindOf(err) != apperr.KindProviderOutput {
		t.Fatalf("kind = %q, want provider output", apperr.KindOf(err))
	}
}

func TestDecodeErrorsNeverEchoRawModelOutput(t *testing.T) {
	t.Parallel()
	secret := "SECRET_RAW_MODEL_OUTPUT_SENTINEL"
	inputs := []string{
		`{"status":"` + secret + `","question":"where?"}`,
		`{"status":"ok","command":"pwd","explanation":"` + secret + `"`,
		secret,
	}
	for _, input := range inputs {
		_, err := DecodeResult([]byte(input))
		if err == nil {
			t.Fatalf("invalid input %q unexpectedly accepted", input)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(apperr.Message(err), secret) {
			t.Fatalf("raw provider output leaked: %v", err)
		}
	}
}
