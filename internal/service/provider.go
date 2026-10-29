package service

import (
	"context"
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
)

func (s *Service) selectModel(
	last *schema.SessionModel,
	kind schema.ModelKind,
	levels ...schema.ModelLevel,
) (*schema.Model, error) {
	if len(levels) == 0 {
		// Prefer the least intelligent available model, even when a cached model exists.
		return s.store.RandomModel(context.Background(), kind)
	}

	if last != nil && !last.ModelID.IsZero() && !last.UsedAt.IsZero() &&
		time.Now().Sub(last.UsedAt) < schema.ModelCacheTTL {

		m, err := s.store.GetModel(bson.M{
			"_id":    last.ModelID,
			"enable": true,
			"type":   kind,
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		if err == nil {
			for _, item := range m.ApiKeys {
				if item != nil && item.ID == last.APIKeyID && item.Enable && item.Weight > 0 {
					return &schema.Model{
						ID:       m.ID,
						Provider: m.Provider,
						Name:     m.Name,
						Url:      m.Url,
						Type:     m.Type,
						Seq:      m.Seq,
						ApiKeys:  []*schema.ApiKey{item},
						Options:  m.Options,
						Enable:   m.Enable,
						Level:    m.Level,
					}, nil
				}
			}
		}
	}
	return s.store.RandomModel(context.Background(), kind, levels...)
}

func (s *Service) newModel(rsp *schema.Model) (model.LLM, error) {
	apiKey := rsp.ApiKeys[0]
	key, err := s.kms.Decrypt(apiKey.Version, apiKey.Ciphertext, apiKey.Nonce)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return llmModel, nil
}
