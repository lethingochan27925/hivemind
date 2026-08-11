// scorer.go: goi Scoring service qua HTTP de lay risk_score, khong load model
// truc tiep trong Go. Khi target la Lambda Function URL dung auth AWS_IAM,
// request bat buoc phai duoc ky SigV4 - neu khong AWS tra 403 truoc khi ham chay.
package scorer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	// scoring-python nap pandas/sklearn/xgboost nen cold start co the toi ~25s.
	// Timeout nay phai NHO HON timeout cua chinh Lambda scoring-api, neu khong
	// Lambda bi giet truoc va loi tra ve khong noi len duoc nguyen nhan.
	// Khi chay tren Lambda, context cua request con siet them mot lan nua.
	httpTimeout    = 45 * time.Second
	signingService = "lambda"
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

// Client goi scoring service. signer == nil nghia la endpoint khong yeu cau
// xac thuc (chay local), khac nil thi moi request duoc ky bang credential cua
// execution role.
type Client struct {
	endpoint string
	http     *http.Client
	signer   *sigv4.Signer
	creds    aws.CredentialsProvider
	region   string
}

// NewClient tao client khong ky - dung cho local dev, noi service Python chay
// truc tiep tren localhost va khong co IAM o giua.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: httpTimeout},
	}
}

// NewSignedClient tao client ky SigV4 bang credential cua AWS config truyen vao.
// Nho vay Function URL cua scoring-python giu duoc auth AWS_IAM: endpoint khong
// public, chi principal co quyen lambda:InvokeFunctionUrl moi goi duoc.
func NewSignedClient(cfg aws.Config, endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: httpTimeout},
		signer:   sigv4.NewSigner(),
		creds:    cfg.Credentials,
		region:   cfg.Region,
	}
}

func (c *Client) Score(ctx context.Context, req ScoreRequest) (float64, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshaling score request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("building score request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if err := c.sign(ctx, httpReq, body); err != nil {
		return 0, err
	}

	resp, err := c.http.Do(httpReq)
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

// sign ky request theo SigV4 cho service "lambda". Payload hash phai khop chinh
// xac body da gui, neu lech thi AWS tu choi chu ky.
func (c *Client) sign(ctx context.Context, req *http.Request, body []byte) error {
	if c.signer == nil {
		return nil
	}

	creds, err := c.creds.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("retrieving credentials for signing: %w", err)
	}

	sum := sha256.Sum256(body)
	if err := c.signer.SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]),
		signingService, c.region, time.Now()); err != nil {
		return fmt.Errorf("signing score request: %w", err)
	}
	return nil
}
