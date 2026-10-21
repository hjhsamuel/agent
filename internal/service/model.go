package service

import (
	"errors"
	"fmt"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

func (s *Service) AddApiKey(id string, name string, key string, weight int64) (int64, error) {
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
		Name:       name,
		Version:    val.Version,
		Ciphertext: val.Ciphertext,
		Nonce:      val.Nonce,
		Weight:     weight,
		Enable:     false,
	}

	return s.store.AddApiKey(modelId, obj)
}

func (s *Service) GetApiKeys(id string) ([]*schema.ApiKey, error) {
	modelId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	obj, err := s.store.GetModel(
		bson.M{"_id": modelId},
		options.FindOne().SetProjection(bson.M{"api_keys": 1}),
	)
	if err != nil {
		return nil, err
	}

	return obj.ApiKeys, nil
}

func (s *Service) UpdateModel(id string, req *request.UpdateModelReq) error {
	if req == nil {
		return errors.New("no update request")
	}

	modelId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	update := bson.M{}
	if req.Provider != nil {
		update["provider"] = *req.Provider
	}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Url != nil {
		update["url"] = *req.Url
	}
	if req.ContextLimit != nil {
		update["options.context_limit"] = *req.ContextLimit
	}
	if req.Reasoning != nil {
		update["options.reasoning"] = &schema.KeyPair{
			Key: req.Reasoning.Key,
			Val: req.Reasoning.Value,
		}
	}
	if req.Enable != nil {
		update["enable"] = *req.Enable
	}

	if len(update) == 0 {
		return nil
	}

	filter := bson.M{"_id": modelId}
	return s.store.UpdateModel(filter, bson.M{"$set": update})
}

func (s *Service) DeleteApiKey(modelId string, apiKeyId int64) error {
	id, err := bson.ObjectIDFromHex(modelId)
	if err != nil {
		return err
	}

	return s.store.DeleteApiKey(
		bson.M{"_id": id},
		bson.M{"$pull": bson.M{"api_keys": bson.M{"id": apiKeyId}}},
	)
}

func (s *Service) UpdateApiKey(modelId string, apiKeyId int64, req *request.UpdateApiKeyReq) error {
	if req == nil {
		return errors.New("no update request")
	}

	id, err := bson.ObjectIDFromHex(modelId)
	if err != nil {
		return err
	}

	update := bson.M{}
	if req.Name != nil {
		update["api_keys.$.name"] = *req.Name
	}
	if req.Weight != nil {
		update["api_keys.$.weight"] = *req.Weight
	}
	if req.Enable != nil {
		update["api_keys.$.enable"] = *req.Enable
	}

	if req.Upgrade {
		obj, err := s.store.GetModel(
			bson.M{"_id": id, "api_keys.id": apiKeyId},
			options.FindOne().SetProjection(bson.M{"api_keys": bson.M{"$elemMatch": bson.M{"id": apiKeyId}}}),
		)
		if err != nil {
			return err
		}

		if len(obj.ApiKeys) == 0 {
			return errors.New("api key not found")
		}
		keyObj := obj.ApiKeys[0]

		latestVer, _, err := s.kms.GetLatestKey()
		if err != nil {
			return err
		}
		if latestVer != keyObj.Version {
			key, err := s.kms.Decrypt(keyObj.Version, keyObj.Ciphertext, keyObj.Nonce)
			if err != nil {
				return err
			}
			val, err := s.kms.Encrypt(key)
			if err != nil {
				return err
			}

			update["api_keys.$.version"] = val.Version
			update["api_keys.$.ciphertext"] = val.Ciphertext
			update["api_keys.$.nonce"] = val.Nonce
		}
	}

	return s.store.UpdateModel(
		bson.M{"_id": id, "api_keys.id": apiKeyId},
		bson.M{"$set": update},
	)
}
