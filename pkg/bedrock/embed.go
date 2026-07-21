// embed.go: goi Titan Embeddings v2 de sinh vector cho case summary.
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type titanEmbedRequest struct {
	InputText  string `json:"inputText"`
	Dimensions int    `json:"dimensions"`
	Normalize  bool   `json:"normalize"`
}

type titanEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (c *Client) EmbedText(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(titanEmbedRequest{
		InputText:  text,
		Dimensions: c.EmbedDim,
		Normalize:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling embed request: %w", err)
	}

	out, err := c.Embedding.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &c.TitanModelID,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        reqBody,
	})
	if err != nil {
		return nil, fmt.Errorf("invoking titan embed model: %w", err)
	}

	var resp titanEmbedResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling embed response: %w", err)
	}

	return resp.Embedding, nil
}

func strPtr(s string) *string { return &s }

var _ = types.ResponseStreamMemberChunk{}
