// reasoning.go: build prompt va goi Claude Haiku de sinh verdict cho 1 giao dich.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/lethingochan27925/hivemind/pkg/bedrock"
	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

type ReasoningResult struct {
	Verdict    string
	Confidence float64
	Rationale  string
	Step       string
	TokensIn   *int
	TokensOut  *int
	LatencyMs  int
}

type claudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	Messages         []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type verdictJSON struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

func BuildPrompt(txn *mcp.Transaction, memoryHits []string, customerHistory []string) string {
	var memoryContext, customerContext string

	if len(memoryHits) > 0 {
		memoryContext = fmt.Sprintf("\nSimilar past cases (reference only, do NOT anchor to these):\n%s\n",
			strings.Join(memoryHits, "\n"))
	}
	if len(customerHistory) > 0 {
		customerContext = fmt.Sprintf("\nThis customer's recent transaction history:\n%s\n",
			strings.Join(customerHistory, "\n"))
	}

	nameOrig := SanitizeField(txn.NameOrig, 64)
	nameDest := SanitizeField(txn.NameDest, 64)

	return fmt.Sprintf(`You are a fraud investigation agent. Analyze this transaction independently.

Transaction:
  type=%s
  amount=%.2f
  name_orig=%s
  name_dest=%s
  risk_score=%.3f
  error_balance_orig=%.2f
  error_balance_dest=%.2f
%s%s
Scoring rules (follow strictly):
- risk_score < 0.30 AND both errors near 0 -> legit
- risk_score 0.30-0.60 AND uncertain signals -> escalate
- risk_score > 0.60 OR large error_balance -> fraud

Respond in JSON only:
{
  "verdict": "fraud" | "escalate" | "legit",
  "confidence": 0.0-1.0,
  "rationale": "one sentence explanation"
}`, txn.Type, txn.Amount, nameOrig, nameDest, txn.RiskScore(),
		txn.ErrorBalanceOrig, txn.ErrorBalanceDest, memoryContext, customerContext)
}

func CallClaude(ctx context.Context, client *bedrock.Client, txn *mcp.Transaction, memoryHits, customerHistory []string) ReasoningResult {
	prompt := BuildPrompt(txn, memoryHits, customerHistory)
	start := time.Now()

	reqBody, err := json.Marshal(claudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        256,
		Messages:         []claudeMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		fmt.Printf("  [claude][error] marshaling request: %v\n", err)
		return ruleBasedFallback(txn, start)
	}

	out, err := client.Reasoning.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &client.ClaudeModelID,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        reqBody,
	})
	if err != nil {
		fmt.Printf("  [claude][error] invoking model: %v\n", err)
		return ruleBasedFallback(txn, start)
	}

	latencyMs := int(time.Since(start).Milliseconds())

	var resp claudeResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		fmt.Printf("  [claude][error] unmarshaling response: %v (body=%s)\n", err, string(out.Body))
		result := ruleBasedFallback(txn, start)
		result.LatencyMs = latencyMs
		return result
	}
	if len(resp.Content) == 0 {
		fmt.Printf("  [claude][error] empty content in response (body=%s)\n", string(out.Body))
		result := ruleBasedFallback(txn, start)
		result.LatencyMs = latencyMs
		return result
	}

	text := strings.TrimSpace(resp.Content[0].Text)
	if strings.Contains(text, "```") {
		parts := strings.Split(text, "```")
		if len(parts) > 1 {
			text = strings.TrimSpace(strings.ReplaceAll(parts[1], "json", ""))
		}
	}

	var v verdictJSON
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		fmt.Printf("  [claude][error] parsing verdict JSON: %v (text=%s)\n", err, text)
		result := ruleBasedFallback(txn, start)
		result.LatencyMs = latencyMs
		return result
	}
	if v.Verdict != "fraud" && v.Verdict != "escalate" && v.Verdict != "legit" {
		fmt.Printf("  [claude][error] invalid verdict value: %q\n", v.Verdict)
		result := ruleBasedFallback(txn, start)
		result.LatencyMs = latencyMs
		return result
	}

	tokensIn := resp.Usage.InputTokens
	tokensOut := resp.Usage.OutputTokens

	return ReasoningResult{
		Verdict:    v.Verdict,
		Confidence: v.Confidence,
		Rationale:  v.Rationale,
		Step:       "bedrock_reasoning",
		TokensIn:   &tokensIn,
		TokensOut:  &tokensOut,
		LatencyMs:  latencyMs,
	}
}

func ruleBasedFallback(txn *mcp.Transaction, start time.Time) ReasoningResult {
	latencyMs := int(time.Since(start).Milliseconds())
	risk := txn.RiskScore()

	switch {
	case risk >= 0.80:
		return ReasoningResult{Verdict: "fraud", Confidence: 0.90, Rationale: "high risk score", Step: "fallback", LatencyMs: latencyMs}
	case risk >= 0.50:
		return ReasoningResult{Verdict: "escalate", Confidence: 0.70, Rationale: "medium risk score", Step: "fallback", LatencyMs: latencyMs}
	default:
		return ReasoningResult{Verdict: "legit", Confidence: 0.85, Rationale: "low risk score", Step: "fallback", LatencyMs: latencyMs}
	}
}

func strPtr(s string) *string { return &s }
