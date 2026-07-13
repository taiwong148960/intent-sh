package protocol

const MaxProviderOutputBytes = 64 * 1024

const (
	ProviderStatusOK      = "ok"
	ProviderStatusClarify = "clarify"
)

// ProviderResult is the strict union returned by an official provider CLI.
type ProviderResult struct {
	Status      string   `json:"status"`
	Command     string   `json:"command,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	Assumptions []string `json:"assumptions,omitempty"`
	RiskHint    string   `json:"riskHint,omitempty"`
	Question    string   `json:"question,omitempty"`
}
