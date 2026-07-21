// client.go: khoi tao Bedrock clients cho reasoning (Claude) va embedding (Titan).
// Hai model co the nam o hai region khac nhau nen can hai client rieng.
package bedrock

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type Client struct {
	Reasoning     *bedrockruntime.Client
	Embedding     *bedrockruntime.Client
	ClaudeModelID string
	TitanModelID  string
	EmbedDim      int
}

func NewClient(ctx context.Context, accessKeyID, secretAccessKey, reasoningRegion, embedRegion, claudeModelID, titanModelID string, embedDim int) (*Client, error) {
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	reasoningCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(reasoningRegion),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("loading reasoning region config: %w", err)
	}

	embedCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(embedRegion),
		awsconfig.WithCredentialsProvider(creds),
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
