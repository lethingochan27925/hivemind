// scorer.go: goi Scoring Lambda qua HTTP de lay risk_score, khong load model truc tiep trong Go.
package scorer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ScoreRequest struct {
	Step             int     `json:"step"`
	Type             string  `json:"type"`
	Amount           float64 `json:"amount"`
	OldBalanceOrig   float64 `json:"oldBalanceOrig"`
	NewBalanceOrig   float64 `json:"newBalanceOrig"`
	OldBalanceDest   float64 `json:"oldBalanceDest"`
	NewBalanceDest   float64 `json:"newBalanceDest"`
	ErrorBalanceOrig float64 `json:"errorBalanceOrig"`
	ErrorBalanceDest float64 `json:"errorBalanceDest"`
}

type ScoreResponse struct {
	RiskScore float64 `json:"risk_score"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Score(req ScoreRequest) (float64, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshaling score request: %w", err)
	}

	resp, err := c.http.Post(c.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("calling scoring api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("scoring api returned %d", resp.StatusCode)
	}

	var scoreResp ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return 0, fmt.Errorf("decoding score response: %w", err)
	}

	return scoreResp.RiskScore, nil
}
