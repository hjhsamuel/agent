package adapter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"strings"
	"sync"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/session"
)

type localState struct {
	mu     sync.RWMutex
	values map[string]any
}

func newLocalState(sessionState, appState, userState schema.ADKState) (*localState, error) {
	s := &localState{values: make(map[string]any, len(sessionState)+len(appState)+len(userState))}
	for _, scope := range []struct {
		prefix string
		state  schema.ADKState
	}{{"", sessionState}, {session.KeyPrefixApp, appState}, {session.KeyPrefixUser, userState}} {
		if err := s.restore(scope.prefix, scope.state); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// restore is used only while constructing an unpublished state.
func (s *localState) restore(prefix string, encoded schema.ADKState) error {
	for key, data := range encoded {
		decoded, err := decodeStateKey(key)
		if err != nil {
			return err
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode state %q: %w", decoded, err)
		}
		s.values[prefix+decoded] = value
	}
	return nil
}

func (s *localState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return value, nil
}

// Set changes the local snapshot; persistence is driven by event state deltas.
func (s *localState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

func (s *localState) All() iter.Seq2[string, any] {
	s.mu.RLock()
	values := maps.Clone(s.values)
	s.mu.RUnlock()
	return maps.All(values)
}

func (s *localState) apply(delta stateDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	maps.Copy(s.values, delta)
}

// stateDelta owns scope partitioning, persistence encoding and temp filtering.
type stateDelta map[string]any

func (d stateDelta) encode() (app, user, local schema.ADKState, err error) {
	app, user, local = make(schema.ADKState), make(schema.ADKState), make(schema.ADKState)
	for key, value := range d {
		target, cleanKey := local, key
		if strings.HasPrefix(key, session.KeyPrefixTemp) {
			continue
		}
		if suffix, ok := strings.CutPrefix(key, session.KeyPrefixApp); ok {
			target, cleanKey = app, suffix
		} else if suffix, ok := strings.CutPrefix(key, session.KeyPrefixUser); ok {
			target, cleanKey = user, suffix
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("encode state %q: %w", key, err)
		}
		target[encodeStateKey(cleanKey)] = data
	}
	return app, user, local, nil
}

func (d stateDelta) persistent() map[string]any {
	if d == nil {
		return nil
	}
	filtered := make(map[string]any, len(d))
	for key, value := range d {
		if !strings.HasPrefix(key, session.KeyPrefixTemp) {
			filtered[key] = value
		}
	}
	return filtered
}

func encodeStateKey(key string) string {
	return "k" + base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeStateKey(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, "k") {
		return "", fmt.Errorf("invalid encoded state key %q", encoded)
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded[1:])
	if err != nil {
		return "", fmt.Errorf("decode state key %q: %w", encoded, err)
	}
	return string(key), nil
}

var _ session.State = (*localState)(nil)
