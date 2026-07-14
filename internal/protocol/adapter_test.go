package protocol

import (
	"bytes"
	"encoding/json"
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
		EditorBackend:   EditorBackendZLE,
		EditorVersion:   "5.9",
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
		{"not terminated", "2\x00rewrite", apperr.KindProtocol},
		{"protocol 1 field count", requestFrame("1", "rewrite", "zsh", "5.9", "x", "0", "", "", "0", "id"), apperr.KindProtocol},
		{"bad version", requestFrame("9", "rewrite", "zsh", "5.9", "zle", "5.9", "x", "0", "", "", "0", "id"), apperr.KindProtocol},
		{"bad cursor", requestFrame("2", "rewrite", "zsh", "5.9", "zle", "5.9", "x", "nope", "", "", "0", "id"), apperr.KindProtocol},
		{"bad generation", requestFrame("2", "rewrite", "zsh", "5.9", "zle", "5.9", "x", "0", "", "", "-1", "id"), apperr.KindProtocol},
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

func TestAdapterRequestJSONIncludesEditorMetadata(t *testing.T) {
	req := AdapterRequest{
		Version: AdapterVersion, Action: ActionRewrite, Shell: "bash", ShellVersion: "3.2.57(1)-release",
		EditorBackend: EditorBackendBlesh, EditorVersion: BleshVersion, Buffer: "列出文件", Cursor: len("列出"), RequestID: "json-1",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"editorBackend":"blesh"`, `"editorVersion":"` + BleshVersion + `"`, `"cursor":6`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("adapter JSON %q does not contain %q", data, want)
		}
	}
}

func TestUTF8CursorConversionAndValidation(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		characters int
		bytes      int
	}{
		{name: "ASCII", buffer: "abc", characters: 2, bytes: 2},
		{name: "Chinese", buffer: "a中b", characters: 2, bytes: 4},
		{name: "combining character", buffer: "e\u0301x", characters: 2, bytes: 3},
		{name: "end", buffer: "列出", characters: 2, bytes: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			offset, err := UTF8ByteOffset(test.buffer, test.characters)
			if err != nil {
				t.Fatalf("UTF8ByteOffset() error = %v", err)
			}
			if offset != test.bytes {
				t.Fatalf("offset = %d, want %d", offset, test.bytes)
			}
			if err := ValidateUTF8ByteCursor(test.buffer, offset); err != nil {
				t.Fatalf("ValidateUTF8ByteCursor() error = %v", err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		buffer string
		cursor int
	}{
		{name: "inside Chinese rune", buffer: "a中b", cursor: 2},
		{name: "inside combining mark", buffer: "e\u0301x", cursor: 2},
		{name: "past end", buffer: "abc", cursor: 4},
		{name: "invalid UTF-8", buffer: string([]byte{'a', 0xff}), cursor: 1},
	} {
		t.Run("reject "+test.name, func(t *testing.T) {
			if err := ValidateUTF8ByteCursor(test.buffer, test.cursor); err == nil {
				t.Fatal("expected invalid cursor error")
			}
		})
	}
	if _, err := UTF8ByteOffset("abc", 4); err == nil {
		t.Fatal("expected out-of-range character cursor error")
	}
}

func requestFrame(fields ...string) string {
	return strings.Join(fields, "\x00") + "\x00"
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
