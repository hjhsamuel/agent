package service

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/llm/openaix"
	"google.golang.org/adk/v2/model"
)

func (s *Service) randomModel(kind schema.ModelKind) (model.LLM, error) {
	rsp, err := s.store.RandomModel(context.Background(), kind)
	if err != nil {
		return nil, err
	}

	apiKey := rsp.ApiKeys[0]
	key, err := s.kms.Decrypt(apiKey.Version, apiKey.Ciphertext, apiKey.Nonce)
	if err != nil {
		return nil, err
	}

	llmModel, err := openaix.NewModel(
		context.Background(),
		rsp.Name,
		&openaix.Config{
			ApiKey: key,
			Url:    rsp.Url,
			API:    openaix.APIChatCompletions,
		},
	)
	if err != nil {
		return nil, err
	}

	return llmModel, nil
}
