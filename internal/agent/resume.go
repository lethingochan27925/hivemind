// resume.go: doc scratchpad de agent tiep tuc dung buoc sau khi bi crash/re-queue.
package agent

import "encoding/json"

type Scratchpad struct {
	MCPResult        json.RawMessage `json:"mcp_result,omitempty"`
	TopKCases        json.RawMessage `json:"top_k_cases,omitempty"`
	PartialReasoning string          `json:"partial_reasoning,omitempty"`
	RetryCount       int             `json:"retry_count"`
}

func ParseScratchpad(raw []byte) (*Scratchpad, error) {
	if len(raw) == 0 {
		return &Scratchpad{}, nil
	}
	var sp Scratchpad
	if err := json.Unmarshal(raw, &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

func (sp *Scratchpad) Encode() ([]byte, error) {
	return json.Marshal(sp)
}
