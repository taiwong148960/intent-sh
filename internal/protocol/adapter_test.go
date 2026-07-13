package protocol

import (
	"bytes"
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

func TestAdapterRequestRoundTrip(t *testing.T) {
	want := AdapterRequest{
		Version:         AdapterVersion,
		Action:          ActionRewrite,
		Shell:           "zsh",
		ShellVersion:    "5.9",
		Buffer:          "printf 'first\\nsecond'\n# editable newline",
		Cursor:          7,
		Original:        "列出文件",
		Previous:        "ls -la",
		GenerationIndex: 2,
		RequestID:       "req-123",
	}
	var frame bytes.Buffer
	if err := EncodeRequest(&frame, want); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRequest(&frame)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestAdapterResponseRoundTrip(t *testing.T) {
	want := AdapterResponse{Version: AdapterVersion, Status: StatusOK, Replacement: "ls -la", Provider: "codex", Risk: "safe", RequestID: "req-1"}
	var frame bytes.Buffer
	if err := EncodeResponse(&frame, want); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResponse(&frame)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestAdapterFrameFailures(t *testing.T) {
	tests := []struct {
		name string
		data string
		kind apperr.Kind
	}{
		{"not terminated", "1\x00rewrite", apperr.KindProtocol},
		{"wrong count", "1\x00rewrite\x00", apperr.KindProtocol},
		{"bad version", "9\x00rewrite\x00zsh\x005.9\x00x\x000\x00\x00\x000\x00id\x00", apperr.KindProtocol},
		{"bad cursor", "1\x00rewrite\x00zsh\x005.9\x00x\x00nope\x00\x00\x000\x00id\x00", apperr.KindProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeRequest(strings.NewReader(tt.data))
			if err == nil || apperr.KindOf(err) != tt.kind {
				t.Fatalf("error = %v, kind = %s", err, apperr.KindOf(err))
			}
		})
	}
}

func TestAdapterFrameBoundsAndNUL(t *testing.T) {
	req := AdapterRequest{Version: AdapterVersion, Action: ActionRewrite, Buffer: strings.Repeat("x", MaxFieldBytes+1)}
	if err := EncodeRequest(&bytes.Buffer{}, req); err == nil {
		t.Fatal("expected oversized field error")
	}
	req.Buffer = "bad\x00field"
	if err := EncodeRequest(&bytes.Buffer{}, req); err == nil {
		t.Fatal("expected NUL error")
	}
	oversized := strings.Repeat("x", MaxFrameBytes+1)
	if _, err := DecodeRequest(strings.NewReader(oversized)); err == nil {
		t.Fatal("expected oversized frame error")
	}
}
