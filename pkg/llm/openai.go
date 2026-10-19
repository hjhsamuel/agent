package llm

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type OpenAI struct {
	client openai.Client
}

func (o *OpenAI) Chat(
	ctx context.Context,
	model string,
	messages ...openai.ChatCompletionMessageParamUnion,
) (*openai.ChatCompletion, error) {
	return o.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    model,
			Messages: messages,
		},
	)
}

func NewOpenAI(path, apiKey string) *OpenAI {
	client := openai.NewClient(option.WithBaseURL(path), option.WithAPIKey(apiKey))
	return &OpenAI{
		client: client,
	}
}
