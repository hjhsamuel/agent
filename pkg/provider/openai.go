package provider

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type OpenAI struct {
	addr   string
	apiKey string

	client openai.Client
}

func (o *OpenAI) init() error {
	client := openai.NewClient(option.WithBaseURL(o.addr), option.WithAPIKey(o.apiKey))
	o.client = client
	return nil
}

func (o *OpenAI) buildParams(
	model string,
	prompt string,
	messages []*Message,
	conf *ChatConfig,
) openai.ChatCompletionNewParams {
	llmMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages)+1)
	llmMessages[0] = openai.SystemMessage(prompt)
	for i, item := range messages {
		var message openai.ChatCompletionMessageParamUnion
		switch item.Role {
		case RoleSystem:
			message = openai.SystemMessage(item.Content)
		case RoleUser:
			message = openai.UserMessage(item.Content)
		case RoleAssistant:
			message = openai.AssistantMessage(item.Content)
			for _, toolCall := range item.ToolCalls {
				message.OfAssistant.ToolCalls = append(message.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: toolCall.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      toolCall.Name,
							Arguments: toolCall.Arguments,
						},
					},
				})
			}
		case RoleTool:
			message = openai.ToolMessage(item.Content, item.ToolCallId)
		default:
			continue
		}
		llmMessages[i+1] = message
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: llmMessages,
	}
	if conf != nil {
		params.ResponseFormat = conf.ResponseFormat

		if conf.Temperature != 0 {
			params.Temperature = openai.Float(conf.Temperature)
		}
		if len(conf.Tool) != 0 {
			params.Tools = conf.Tool
		}
		if len(conf.Extra) != 0 {
			params.SetExtraFields(conf.Extra)
		}
	}

	return params
}

func (o *OpenAI) Chat(
	ctx context.Context,
	model string,
	prompt string,
	messages []*Message,
	conf *ChatConfig,
) (*Message, error) {
	params := o.buildParams(model, prompt, messages, conf)
	response, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	out := &Message{
		Role: RoleAssistant,
		Usage: &TokenUsage{
			Total:       response.Usage.TotalTokens,
			Prompt:      response.Usage.PromptTokens,
			Cached:      response.Usage.PromptTokensDetails.CachedTokens,
			Completions: response.Usage.CompletionTokens,
			Reasoning:   response.Usage.CompletionTokensDetails.ReasoningTokens,
		},
	}

	if len(response.Choices) == 0 {
		return out, nil
	}

	choice := response.Choices[0]
	for _, item := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, &ToolCall{
			ID:        item.ID,
			Name:      item.Function.Name,
			Arguments: item.Function.Arguments,
		})
	}

	out.Content = choice.Message.Content

	if v, ok := choice.Message.JSON.ExtraFields["reasoning_content"]; ok {
		out.Reasoning = v.Raw()
	}

	return out, nil
}

func (o *OpenAI) Stream(
	ctx context.Context,
	model string,
	prompt string,
	messages []*Message,
	conf *ChatConfig,
	yield YieldFunc,
) (*Message, error) {
	params := o.buildParams(model, prompt, messages, conf)
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
	stream := o.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			if err := yield(&StreamChunk{Type: Completion, Content: delta.Content}, nil); err != nil {
				return nil, err
			}
		}
		if v, ok := delta.JSON.ExtraFields["reasoning_content"]; ok {
			if err := yield(&StreamChunk{Type: Reasoning, Content: v.Raw()}, nil); err != nil {
				return nil, err
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	message := &Message{
		Role: RoleAssistant,
		Usage: &TokenUsage{
			Total:       acc.Usage.TotalTokens,
			Prompt:      acc.Usage.PromptTokens,
			Cached:      acc.Usage.PromptTokensDetails.CachedTokens,
			Completions: acc.Usage.CompletionTokens,
			Reasoning:   acc.Usage.CompletionTokensDetails.ReasoningTokens,
		},
	}
	if len(acc.Choices) != 0 {
		m := acc.Choices[0].Message
		message.Content = m.Content
		if v, ok := m.JSON.ExtraFields["reasoning_content"]; ok {
			message.Reasoning = v.Raw()
		}
		for _, item := range m.ToolCalls {
			message.ToolCalls = append(message.ToolCalls, &ToolCall{
				ID:        item.ID,
				Name:      item.Function.Name,
				Arguments: item.Function.Arguments,
			})
		}
	}

	return message, nil
}

func NewOpenAI(addr, apiKey string) (*OpenAI, error) {
	o := &OpenAI{
		addr:   addr,
		apiKey: apiKey,
	}
	if err := o.init(); err != nil {
		return nil, err
	}
	return o, nil
}
