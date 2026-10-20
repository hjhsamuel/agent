package service

import (
	"fmt"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) AddModel(
	provider string,
	modelName string,
	baseUrl string,
	kind string,
	contextLimit int64,
	reasonKey string,
	reasonValue string,
) (string, error) {
	modelObj := &schema.Model{
		Provider: provider,
		Name:     modelName,
		Url:      baseUrl,
		Type:     schema.ModelKind(kind),
		Seq:      1,
		ApiKeys:  make([]*schema.ApiKey, 0),
		Options: schema.ModelConfig{
			ContextLimit: contextLimit,
		},
		Enable: false,
	}
	if reasonKey != "" && reasonValue != "" {
		modelObj.Options.Reasoning = &schema.KeyPair{
			Key: reasonKey,
			Val: reasonValue,
		}
	}

	modelId, err := s.store.AddModel(modelObj)
	if err != nil {
		return "", err
	}

	return modelId.Hex(), nil
}

func (s *Service) GetModelGroups() ([]*request.GroupModel, error) {
	return s.store.GroupModel()
}

func (s *Service) AddApiKey(id string, key string, weight int64) (int64, error) {
	if weight < schema.MinApiKeyWeight || weight > schema.MaxApiKeyWeight {
		return 0, fmt.Errorf("weight out of range: [%d, %d]", schema.MinApiKeyWeight, schema.MaxApiKeyWeight)
	}

	modelId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return 0, err
	}

	val, err := s.kms.Encrypt(key)
	if err != nil {
		return 0, err
	}

	obj := &schema.ApiKey{
		Version:    val.Version,
		Ciphertext: val.Ciphertext,
		Nonce:      val.Nonce,
		Weight:     weight,
		Enable:     false,
	}

	return s.store.AddApiKey(modelId, obj)
}
