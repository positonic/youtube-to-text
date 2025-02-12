package embeddings

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// GetEmbedding converts text to an embedding vector using OpenAI's API
func GetEmbedding(text string, apiKey string) ([]float32, error) {
	client := openai.NewClient(apiKey)
	
	resp, err := client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Model: openai.AdaEmbeddingV2,
		Input: []string{text},
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding creation failed: %w", err)
	}
	
	return resp.Data[0].Embedding, nil
} 