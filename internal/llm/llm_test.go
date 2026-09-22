package llm

import (
	"context"
	"testing"

	"github.com/hjhsamuel/agent/pkg/provider"
)

type emptyProvider struct{}

func (emptyProvider) Chat(context.Context, string, string, []*provider.Message, *provider.ChatConfig) (*provider.Message, error) {
	return nil, nil
}
func (emptyProvider) Stream(context.Context, string, string, []*provider.Message, *provider.ChatConfig, provider.YieldFunc) (*provider.Message, error) {
	return nil, nil
}

func TestEmptyProviderResponseIsAnError(t *testing.T) {
	m := New("test", nil, emptyProvider{})
	if _, err := m.Chat(context.Background(), "", nil, nil); err == nil {
		t.Fatal("nil chat response accepted")
	}
	if _, err := m.Stream(context.Background(), "", nil, nil, nil); err == nil {
		t.Fatal("nil stream response accepted")
	}
}
