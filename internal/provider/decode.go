package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

const (
	maxExplanationBytes = 1024
	maxQuestionBytes    = 1024
	maxAssumptions      = 16
	maxAssumptionBytes  = 256
)

var (
	errDuplicateJSONField = errors.New("duplicate JSON object field")
	errJSONNestingLimit   = errors.New("JSON nesting limit exceeded")
)

type resultWire struct {
	Status      *string   `json:"status"`
	Command     *string   `json:"command"`
	Explanation *string   `json:"explanation"`
	Assumptions *[]string `json:"assumptions"`
	RiskHint    *string   `json:"riskHint"`
	Question    *string   `json:"question"`
}

// DecodeResult accepts exactly one bounded JSON object matching one union branch.
func DecodeResult(data []byte) (protocol.ProviderResult, error) {
	if len(data) == 0 {
		return protocol.ProviderResult{}, outputError("provider returned no structured result")
	}
	if len(data) > protocol.MaxProviderOutputBytes {
		return protocol.ProviderResult{}, outputError("provider result exceeded the output limit")
	}
	if !utf8.Valid(data) {
		return protocol.ProviderResult{}, outputError("provider result was not valid UTF-8")
	}
	if err := rejectDuplicateJSONFields(data); err != nil {
		if errors.Is(err, errDuplicateJSONField) {
			return protocol.ProviderResult{}, outputError("provider result contained duplicate object fields")
		}
		return protocol.ProviderResult{}, outputError("provider result was not valid JSON")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire resultWire
	if err := decoder.Decode(&wire); err != nil {
		return protocol.ProviderResult{}, outputWrap("provider result was not valid JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return protocol.ProviderResult{}, outputError("provider returned more than one JSON value")
		}
		return protocol.ProviderResult{}, outputWrap("provider result contained trailing content", err)
	}
	if wire.Status == nil {
		return protocol.ProviderResult{}, outputError("provider result omitted status")
	}

	switch *wire.Status {
	case protocol.ProviderStatusOK:
		if wire.Command == nil || wire.Explanation == nil || wire.Assumptions == nil || wire.RiskHint == nil {
			return protocol.ProviderResult{}, outputError("provider command result omitted required fields")
		}
		if wire.Question != nil {
			return protocol.ProviderResult{}, outputError("provider command result contained clarification fields")
		}
		if len(*wire.Command) == 0 || len(*wire.Command) > 8192 {
			return protocol.ProviderResult{}, outputError("provider command had an invalid length")
		}
		if len(*wire.Explanation) > maxExplanationBytes {
			return protocol.ProviderResult{}, outputError("provider explanation exceeded the output limit")
		}
		if len(*wire.Assumptions) > maxAssumptions {
			return protocol.ProviderResult{}, outputError("provider returned too many assumptions")
		}
		for _, assumption := range *wire.Assumptions {
			if len(assumption) > maxAssumptionBytes {
				return protocol.ProviderResult{}, outputError("provider assumption exceeded the output limit")
			}
		}
		if !validRiskHint(*wire.RiskHint) {
			return protocol.ProviderResult{}, outputError("provider returned an invalid risk hint")
		}
		return protocol.ProviderResult{
			Status:      *wire.Status,
			Command:     *wire.Command,
			Explanation: *wire.Explanation,
			Assumptions: append([]string(nil), (*wire.Assumptions)...),
			RiskHint:    *wire.RiskHint,
		}, nil

	case protocol.ProviderStatusClarify:
		if wire.Question == nil || len(*wire.Question) == 0 || len(*wire.Question) > maxQuestionBytes {
			return protocol.ProviderResult{}, outputError("provider clarification had an invalid question")
		}
		if wire.Command != nil || wire.Explanation != nil || wire.Assumptions != nil || wire.RiskHint != nil {
			return protocol.ProviderResult{}, outputError("provider clarification contained command fields")
		}
		return protocol.ProviderResult{Status: *wire.Status, Question: *wire.Question}, nil

	default:
		return protocol.ProviderResult{}, outputError("provider returned an unsupported status")
	}
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	return scanJSONValue(decoder, 0)
}

func scanJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errJSONNestingLimit
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key was not a string")
			}
			if _, exists := seen[key]; exists {
				return errDuplicateJSONField
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func validRiskHint(value string) bool {
	return value == "safe" || value == "review" || value == "dangerous"
}

func outputError(message string) error {
	return apperr.New(apperr.KindProviderOutput, "decode provider result", message)
}

func outputWrap(message string, err error) error {
	return apperr.Wrap(apperr.KindProviderOutput, "decode provider result", message, err)
}

func boundedText(value string, limit int) string {
	return textsafe.Terminal(value, limit)
}
