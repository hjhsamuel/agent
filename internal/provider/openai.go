package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type OpenAI struct {
	path       string
	apiKey     string
	dimensions int64

	client openai.Client
}

func (o *OpenAI) init() {
	o.client = openai.NewClient(
		option.WithBaseURL(o.path),
		option.WithAPIKey(o.apiKey),
	)
}

func (o *OpenAI) Chat(ctx context.Context, req *ChatReq, onChunk OnChunk) (*ChatRsp, error) {
	params := o.parseChatParams(req)

	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	out := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		if onChunk != nil {
			for _, item := range chunk.Choices {
				if item.Delta.Content != "" {
					onChunk(item.Delta.Content)
				}
			}
		}
		if !out.AddChunk(chunk) {
			return nil, errors.New("accumulate chunks failed")
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream chat completion: %v", err)
	}
	if len(out.Choices) == 0 {
		return nil, errors.New("provider returned no choices")
	}

	rsp := o.formatChatResponse(out.Choices[0].Message, out.Choices[0].FinishReason)
	return rsp, nil
}

func (o *OpenAI) parseChatParams(req *ChatReq) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model: req.Model,
	}

	if req.MaxTokens != 0 {
		params.MaxCompletionTokens = openai.Int(req.MaxTokens)
	}

	if req.SystemPrompt != "" {
		params.Messages = append(params.Messages, openai.SystemMessage(req.SystemPrompt))
	}
	for _, message := range req.Messages {
		switch message.Role {
		case SystemMessage:
			params.Messages = append(params.Messages, openai.SystemMessage(message.Content))
		case UserMessage:
			params.Messages = append(params.Messages, openai.UserMessage(message.Content))
		case AssistantMessage:
			msg := openai.AssistantMessage(message.Content)
			for _, item := range message.ToolCalls {
				msg.OfAssistant.ToolCalls = append(msg.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: item.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      item.Name,
							Arguments: string(item.Arguments),
						},
					},
				})
			}
			params.Messages = append(params.Messages, msg)
		case ToolMessage:
			params.Messages = append(params.Messages, openai.ToolMessage(message.Content, message.ToolCallID))
		}
	}

	for _, tool := range req.Tools {
		params.Tools = append(params.Tools, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        tool.Name,
					Description: openai.String(tool.Description),
					Parameters:  tool.Parameter,
				},
			},
		})
	}

	return params
}

func (o *OpenAI) formatChatResponse(msg openai.ChatCompletionMessage, reason string) *ChatRsp {
	message := &Message{
		Role:    AssistantMessage,
		Content: msg.Content,
	}
	for _, item := range msg.ToolCalls {
		if item.Type != "function" {
			continue
		}
		message.ToolCalls = append(message.ToolCalls, &ToolCall{
			ID:        item.ID,
			Name:      item.Function.Name,
			Arguments: []byte(item.Function.Arguments),
		})
	}
	return &ChatRsp{
		Message:      message,
		FinishReason: reason,
	}
}

func (o *OpenAI) Embedding(ctx context.Context, model string, messages []string) ([][]float64, error) {
	if len(messages) == 0 {
		return [][]float64{}, nil
	}

	params := openai.EmbeddingNewParams{
		Model: model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: messages,
		},
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	}
	if o.dimensions > 0 {
		params.Dimensions = openai.Int(o.dimensions)
	}

	rsp, err := o.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create embedding: %v", err)
	}
	if len(rsp.Data) != len(messages) {
		return nil, fmt.Errorf("create embedding: expected %d messages, got %d", len(messages), len(rsp.Data))
	}

	sort.Slice(rsp.Data, func(i, j int) bool {
		return rsp.Data[i].Index < rsp.Data[j].Index
	})

	vectors := make([][]float64, len(rsp.Data))
	for i, item := range rsp.Data {
		vectors[i] = item.Embedding
	}
	return vectors, nil
}

func NewOpenAI(path string, apiKey string) Provider {
	o := &OpenAI{
		path:   path,
		apiKey: apiKey,
	}
	o.init()
	return o
}
