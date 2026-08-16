// reasoning.go: build prompt va goi Claude Haiku de sinh verdict cho 1 giao dich.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// balanceSignal la cong cu doi soat so du. So hoc chinh xac lam trong Go thay vi
// bat LLM tu tinh (model nho lam kem viec nay). Tra ve mot tin hieu PHAN LOAI de
// agent suy luan - thu ma model nho xu ly tot hon con so tho.
func balanceSignal(txn *mcp.Transaction) string {
	drained := math.Abs(txn.OldBalanceOrig-txn.Amount) < 1.0 && txn.NewBalanceOrig < 1.0
	destCredited := txn.NewBalanceDest >= txn.OldBalanceDest+txn.Amount-1.0
	switch {
	case drained && !destCredited:
		return "DRAIN -- the origin held almost exactly the amount and was emptied to zero, " +
			"and the destination balance did not rise to receive it. The money left the origin and vanished."
	case destCredited:
		return "FUNDS MOVED -- the destination balance rose by about the amount; the money genuinely arrived."
	default:
		return "INCONCLUSIVE -- the balances match neither a clean account drain nor a completed transfer."
	}
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
	balanceCheck := balanceSignal(txn)

	return fmt.Sprintf(`You are a fraud investigation agent reviewing a transaction an upstream model could not
clear automatically. Decide: fraud (auto-block), legit (auto-approve), or escalate (human).

A balance-reconciliation tool already traced the money for you. Its finding is your PRIMARY
evidence -- follow it unless the other signals strongly contradict it:
  >> %s

Mapping:
- finding starts with DRAIN        -> account-drain fraud signature. verdict = fraud.
- finding starts with FUNDS MOVED  -> the money genuinely arrived.   verdict = legit.
- finding starts with INCONCLUSIVE -> you cannot confirm the path.   verdict = escalate.

Transaction (for context):
  type=%s
  amount=%.2f
  name_orig=%s  old_balance_orig=%.2f  new_balance_orig=%.2f
  name_dest=%s  old_balance_dest=%.2f  new_balance_dest=%.2f
  risk_score=%.3f
%s%s
Respond in JSON only:
{
  "verdict": "fraud" | "escalate" | "legit",
  "confidence": 0.0-1.0,
  "rationale": "one sentence explanation"
}`, balanceCheck, txn.Type, txn.Amount, nameOrig, txn.OldBalanceOrig, txn.NewBalanceOrig,
		nameDest, txn.OldBalanceDest, txn.NewBalanceDest, txn.RiskScore(), memoryContext, customerContext)
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
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
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
