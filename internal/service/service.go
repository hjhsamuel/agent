package service

import (
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/pkg/kms"
)

type Service struct {
	store *store.Store
	kms   *kms.KMS
}
