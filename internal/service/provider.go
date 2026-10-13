package service

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
)

func (s *Service) randomModel(kind schema.ModelKind) (*schema.Model, model.LLM, error) {
	rsp, err := s.store.RandomModel(context.Background(), kind)
	if err != nil {
		return nil, nil, err
	}

	apiKey := rsp.ApiKeys[0]
	key, err := s.kms.Decrypt(apiKey.Version, apiKey.Ciphertext, apiKey.Nonce)
	if err != nil {
		return nil, nil, err
	}

	llmModel, err := openaimodel.NewModel(
		context.Background(),
		rsp.Name,
		&openaimodel.ClientConfig{
			APIKey:  key,
			BaseURL: rsp.Url,
			API:     openaimodel.APIChatCompletions,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return rsp, llmModel, nil
}
