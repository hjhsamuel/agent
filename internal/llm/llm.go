package llm

import (
	"context"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/backoff"
	"github.com/hjhsamuel/agent/pkg/provider"
)

type LLM struct {
	Model        string
	Capabilities *schema.ModelCapabilities

	provider provider.Provider
}

func New(model string, capabilities *schema.ModelCapabilities, implementation provider.Provider) *LLM {
	return &LLM{Model: model, Capabilities: capabilities, provider: implementation}
}

func (l *LLM) Chat(
	ctx context.Context,
	prompt string,
	messages []*provider.Message,
	conf *provider.ChatConfig,
) (*provider.Message, error) {
	var (
		response *provider.Message
		err      error
	)
	retry := backoff.NewBackoff(time.Second*3, time.Minute, 0.2)
	for attempt := 0; attempt < 3; attempt++ {
		response, err = l.provider.Chat(ctx, l.Model, prompt, messages, conf)
		if err != nil {
			wErr := backoff.Wait(ctx, retry.Delay(attempt))
			if wErr != nil {
				return nil, wErr
			}
			continue
		}
		return response, nil
	}
	return nil, err
}

func (l *LLM) Stream(
	ctx context.Context,
	prompt string,
	messages []*provider.Message,
	conf *provider.ChatConfig,
	yield provider.YieldFunc,
) (*provider.Message, error) {
	var (
		response *provider.Message
		err      error
	)
	retry := backoff.NewBackoff(time.Second*3, time.Minute, 0.2)
	for attempt := 0; attempt < 3; attempt++ {
		response, err = l.provider.Stream(ctx, l.Model, prompt, messages, conf, yield)
		if err != nil {
			if yieldErr := yield(nil, err); yieldErr != nil {
				return nil, yieldErr
			}
			wErr := backoff.Wait(ctx, retry.Delay(attempt))
			if wErr != nil {
				return nil, wErr
			}
			continue
		}
		return response, nil
	}
	return nil, err
}
