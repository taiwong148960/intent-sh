package schemas_test

import (
	"encoding/json"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/schemas"
)

func TestProviderSchemaUsesStructuredOutputCompatibleRoot(t *testing.T) {
	t.Parallel()
	var document map[string]any
	if err := json.Unmarshal(schemas.ProviderResult, &document); err != nil {
		t.Fatalf("embedded schema is invalid JSON: %v", err)
	}
	if document["type"] != "object" || document["additionalProperties"] != false {
		t.Fatalf("schema root must be a closed object: %#v", document)
	}
	for _, unsupported := range []string{"oneOf", "anyOf", "allOf"} {
		if _, exists := document[unsupported]; exists {
			t.Fatalf("schema root uses unsupported union keyword %q", unsupported)
		}
	}
	required, ok := document["required"].([]any)
	if !ok || len(required) != 6 {
		t.Fatalf("all transport fields must be required: %#v", document["required"])
	}
}

func TestNullableTransportDecodesToStrictLocalUnion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		status string
	}{
		{
			name:   "ok",
			input:  `{"status":"ok","command":"pwd","explanation":"prints the directory","assumptions":[],"riskHint":"safe","question":null}`,
			status: protocol.ProviderStatusOK,
		},
		{
			name:   "clarify",
			input:  `{"status":"clarify","command":null,"explanation":null,"assumptions":null,"riskHint":null,"question":"Which directory?"}`,
			status: protocol.ProviderStatusClarify,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := provider.DecodeResult([]byte(test.input))
			if err != nil || value.Status != test.status {
				t.Fatalf("DecodeResult() = %#v, %v", value, err)
			}
		})
	}

	mixed := `{"status":"ok","command":"pwd","explanation":"x","assumptions":[],"riskHint":"safe","question":"also ask"}`
	if _, err := provider.DecodeResult([]byte(mixed)); err == nil {
		t.Fatal("local union accepted mixed command and clarification fields")
	}
}
