// client.go: khoi tao Bedrock clients cho reasoning (Claude) va embedding (Titan).
// Hai model co the nam o hai region khac nhau nen can hai client rieng.
package bedrock

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Fleet 20 worker song song vuot quota TPS cua Bedrock: mac dinh SDK chi thu 3
// lan roi bo cuoc, khien 52% request roi vao nhanh fallback. RetryModeAdaptive
// tu do ty le bi chan va giam toc phia client thay vi lao vao tuong.
const maxRetryAttempts = 8

type Client struct {
	Reasoning     *bedrockruntime.Client
	Embedding     *bedrockruntime.Client
	ClaudeModelID string
	TitanModelID  string
	EmbedDim      int
}

// NewClient dung credential chain mac dinh cua AWS SDK thay vi credential tinh.
// Tren Lambda, credential cua execution role la TAM THOI va bat buoc di kem
// AWS_SESSION_TOKEN. Ban cu tao static provider voi session token rong nen
// Bedrock tra 403 UnrecognizedClientException. Chain mac dinh xu ly dung ca hai
// truong hop: bien moi truong khi chay local, va execution role khi tren Lambda.
func NewClient(ctx context.Context, reasoningRegion, embedRegion, claudeModelID, titanModelID string, embedDim int) (*Client, error) {
	reasoningCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(reasoningRegion),
		awsconfig.WithRetryMaxAttempts(maxRetryAttempts),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		return nil, fmt.Errorf("loading reasoning region config: %w", err)
	}

	embedCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(embedRegion),
		awsconfig.WithRetryMaxAttempts(maxRetryAttempts),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		return nil, fmt.Errorf("loading embed region config: %w", err)
	}

	return &Client{
		Reasoning:     bedrockruntime.NewFromConfig(reasoningCfg),
		Embedding:     bedrockruntime.NewFromConfig(embedCfg),
		ClaudeModelID: claudeModelID,
		TitanModelID:  titanModelID,
		EmbedDim:      embedDim,
	}, nil
}
