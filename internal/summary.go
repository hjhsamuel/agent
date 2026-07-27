package internal

import (
	"context"

	"github.com/hjhsamuel/agent/internal/memory"
	"github.com/hjhsamuel/agent/internal/provider"
)

type Extractor struct {
	provider provider.Provider
	mem      memory.Memory
}

func (e *Extractor) Save(ctx context.Context, userID, sessionID string, messages []*provider.Message) error {

}
