// Package protocol defines versioned adapter and provider contracts.
package protocol

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

const (
	AdapterVersion = "2"
	MaxFieldBytes  = 32 * 1024
	MaxFrameBytes  = 128 * 1024
)

const (
	EditorBackendZLE      = "zle"
	EditorBackendReadline = "readline"
)

const (
	ActionRewrite = "rewrite"
	StatusOK      = "ok"
	StatusClarify = "clarify"
	StatusError   = "error"
	StatusCancel  = "cancelled"
)

// AdapterRequest is sent by a shell adapter in a fixed NUL-framed order.
type AdapterRequest struct {
	Version         string `json:"version"`
	Action          string `json:"action"`
	Shell           string `json:"shell"`
	ShellVersion    string `json:"shellVersion"`
	EditorBackend   string `json:"editorBackend"`
	EditorVersion   string `json:"editorVersion"`
	Buffer          string `json:"buffer"`
	Cursor          int    `json:"cursor"`
	Original        string `json:"original,omitempty"`
	Previous        string `json:"previous,omitempty"`
	GenerationIndex int    `json:"generationIndex"`
	RequestID       string `json:"requestID"`
}

// AdapterResponse is returned only after provider output is fully validated.
type AdapterResponse struct {
	Version     string `json:"version"`
	Status      string `json:"status"`
	Replacement string `json:"replacement,omitempty"`
	Message     string `json:"message,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Risk        string `json:"risk,omitempty"`
	RiskReason  string `json:"riskReason,omitempty"`
	RequestID   string `json:"requestID"`
}

func EncodeRequest(w io.Writer, req AdapterRequest) error {
	return writeFields(w, []string{
		req.Version,
		req.Action,
		req.Shell,
		req.ShellVersion,
		req.EditorBackend,
		req.EditorVersion,
		req.Buffer,
		strconv.Itoa(req.Cursor),
		req.Original,
		req.Previous,
		strconv.Itoa(req.GenerationIndex),
		req.RequestID,
	})
}

func DecodeRequest(r io.Reader) (AdapterRequest, error) {
	fields, err := readFields(r, 12)
	if err != nil {
		return AdapterRequest{}, err
	}
	if fields[0] != AdapterVersion {
		return AdapterRequest{}, apperr.New(apperr.KindProtocol, "decode adapter request", "adapter protocol is incompatible with binary protocol "+AdapterVersion)
	}
	cursor, err := parseNonNegative(fields[7], "cursor")
	if err != nil {
		return AdapterRequest{}, err
	}
	generation, err := parseNonNegative(fields[10], "generation index")
	if err != nil {
		return AdapterRequest{}, err
	}
	return AdapterRequest{
		Version:         fields[0],
		Action:          fields[1],
		Shell:           fields[2],
		ShellVersion:    fields[3],
		EditorBackend:   fields[4],
		EditorVersion:   fields[5],
		Buffer:          fields[6],
		Cursor:          cursor,
		Original:        fields[8],
		Previous:        fields[9],
		GenerationIndex: generation,
		RequestID:       fields[11],
	}, nil
}

// ValidateUTF8ByteCursor verifies that cursor is a byte offset on a UTF-8
// boundary. Protocol 2 uses byte offsets even when an editor stores a logical
// character index locally.
func ValidateUTF8ByteCursor(buffer string, cursor int) error {
	if !utf8.ValidString(buffer) {
		return apperr.New(apperr.KindInvalidInput, "validate adapter cursor", "editable buffer must be valid UTF-8")
	}
	if cursor < 0 || cursor > len(buffer) {
		return apperr.New(apperr.KindInvalidInput, "validate adapter cursor", "cursor is outside the editable buffer")
	}
	if cursor < len(buffer) && !utf8.RuneStart(buffer[cursor]) {
		return apperr.New(apperr.KindInvalidInput, "validate adapter cursor", "cursor must be on a UTF-8 byte boundary")
	}
	return nil
}

// UTF8ByteOffset converts a zero-based Unicode code-point index into the byte
// offset used by protocol 2. Combining marks remain separate code points.
func UTF8ByteOffset(buffer string, characterOffset int) (int, error) {
	if !utf8.ValidString(buffer) {
		return 0, apperr.New(apperr.KindInvalidInput, "convert adapter cursor", "editable buffer must be valid UTF-8")
	}
	if characterOffset < 0 {
		return 0, apperr.New(apperr.KindInvalidInput, "convert adapter cursor", "character cursor must be non-negative")
	}
	index := 0
	for byteOffset := range buffer {
		if index == characterOffset {
			return byteOffset, nil
		}
		index++
	}
	if index == characterOffset {
		return len(buffer), nil
	}
	return 0, apperr.New(apperr.KindInvalidInput, "convert adapter cursor", "character cursor is outside the editable buffer")
}

func EncodeResponse(w io.Writer, response AdapterResponse) error {
	return writeFields(w, []string{
		response.Version,
		response.Status,
		response.Replacement,
		response.Message,
		response.Provider,
		response.Risk,
		response.RiskReason,
		response.RequestID,
	})
}

func DecodeResponse(r io.Reader) (AdapterResponse, error) {
	fields, err := readFields(r, 8)
	if err != nil {
		return AdapterResponse{}, err
	}
	if fields[0] != AdapterVersion {
		return AdapterResponse{}, apperr.New(apperr.KindProtocol, "decode adapter response", "binary protocol is incompatible with adapter protocol "+AdapterVersion)
	}
	return AdapterResponse{
		Version:     fields[0],
		Status:      fields[1],
		Replacement: fields[2],
		Message:     fields[3],
		Provider:    fields[4],
		Risk:        fields[5],
		RiskReason:  fields[6],
		RequestID:   fields[7],
	}, nil
}

func writeFields(w io.Writer, fields []string) error {
	total := 0
	for _, field := range fields {
		if strings.ContainsRune(field, '\x00') {
			return apperr.New(apperr.KindInvalidInput, "encode adapter frame", "adapter fields cannot contain NUL bytes")
		}
		if len(field) > MaxFieldBytes {
			return apperr.New(apperr.KindInvalidInput, "encode adapter frame", "adapter field exceeds the size limit")
		}
		total += len(field) + 1
		if total > MaxFrameBytes {
			return apperr.New(apperr.KindInvalidInput, "encode adapter frame", "adapter frame exceeds the size limit")
		}
		if _, err := io.WriteString(w, field); err != nil {
			return apperr.Wrap(apperr.KindInternal, "encode adapter frame", "could not write adapter frame", err)
		}
		if _, err := w.Write([]byte{0}); err != nil {
			return apperr.Wrap(apperr.KindInternal, "encode adapter frame", "could not write adapter frame", err)
		}
	}
	return nil
}

func readFields(r io.Reader, count int) ([]string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFrameBytes+1))
	if err != nil {
		return nil, apperr.Wrap(apperr.KindProtocol, "decode adapter frame", "could not read adapter frame", err)
	}
	if len(data) > MaxFrameBytes {
		return nil, apperr.New(apperr.KindProtocol, "decode adapter frame", "adapter frame exceeds the size limit")
	}
	if len(data) == 0 || data[len(data)-1] != 0 {
		return nil, apperr.New(apperr.KindProtocol, "decode adapter frame", "adapter frame is not NUL terminated")
	}
	raw := bytes.Split(data[:len(data)-1], []byte{0})
	if len(raw) != count {
		return nil, apperr.New(apperr.KindProtocol, "decode adapter frame", fmt.Sprintf("adapter frame has %d fields; expected %d", len(raw), count))
	}
	fields := make([]string, len(raw))
	for i, field := range raw {
		if len(field) > MaxFieldBytes {
			return nil, apperr.New(apperr.KindProtocol, "decode adapter frame", fmt.Sprintf("adapter field %d exceeds the size limit", i+1))
		}
		fields[i] = string(field)
	}
	return fields, nil
}

func parseNonNegative(value, name string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, apperr.New(apperr.KindProtocol, "decode adapter request", name+" must be a non-negative integer")
	}
	return n, nil
}
