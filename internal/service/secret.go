package service

import (
	"fmt"

	"github.com/hjhsamuel/agent/pkg/filex"
)

func (s *Service) GenerateSecret() (int64, error) {
	version, key, err := s.kms.GenerateKey()
	if err != nil {
		return 0, err
	}

	err = filex.AppendFile(".env", fmt.Sprintf("CAGENT_SECRET_V%d=%s", version, key))
	if err != nil {
		return 0, err
	}

	return version, nil
}
