package service

import (
	"fmt"

	"github.com/hjhsamuel/agent/pkg/filex"
)

func (s *Service) GenerateSecret() (int64, error) {
	version, err := s.kms.GenerateKey(func(version int64, key string) error {
		w := fmt.Sprintf("\nCAGENT_SECRET_V%d=%s\n", version, key)
		return filex.AppendFile(".env", w)
	})
	if err != nil {
		return 0, err
	}

	return version, nil
}
