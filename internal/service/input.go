package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

func waitInputsReady(inputs []*schema.WaitInput) bool {
	if len(inputs) == 0 {
		return false
	}
	for _, input := range inputs {
		if input == nil || input.ID == "" || input.Content == "" {
			return false
		}
	}
	return true
}

func confirmationMessages(obj *schema.Session) []*schema.Message {
	messages := make([]*schema.Message, 0, len(obj.Wait))
	for _, item := range obj.Wait {
		messages = append(messages, &schema.Message{
			Session:   obj.ID,
			Turn:      obj.TurnSeq - 1,
			Role:      schema.RoleUser,
			Content:   fmt.Sprintf("> %s \n\n%s", item.Hint, item.Content),
			CreatedAt: item.UpdatedAt,
		})
	}
	return messages
}

func (s *Service) sessionHistory(ctx context.Context, user, id string) (session.Session, error) {
	rsp, err := s.adkSession.Get(ctx, &session.GetRequest{
		AppName:   schema.RunnerName,
		UserID:    user,
		SessionID: id,
	})
	if errors.Is(err, session.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rsp == nil || rsp.Session == nil {
		return nil, errors.New("missing session history")
	}
	return rsp.Session, nil
}

func inputHistoryCursor(history session.Session) (string, error) {
	if history == nil {
		return "", nil
	}
	events := history.Events()
	if events.Len() == 0 {
		return "", nil
	}
	last := events.At(events.Len() - 1)
	if last == nil || last.ID == "" {
		return "", errors.New("session history event has no ID")
	}
	return last.ID, nil
}

func (s *Service) recoveryInput(ctx context.Context, obj *schema.Session) (*genai.Content, error) {
	if len(obj.Wait) != 0 {
		if !waitInputsReady(obj.Wait) {
			return nil, errors.New("incomplete submitted confirmation input")
		}
	}
	history, err := s.sessionHistory(ctx, obj.User, obj.ID.Hex())
	if err != nil {
		return nil, err
	}
	if len(obj.Wait) != 0 {
		return recoverConfirmationInput(obj.Wait, history)
	}
	message, err := s.store.GetTurnUserMessage(obj.ID, obj.TurnSeq-1)
	if err != nil {
		return nil, err
	}
	return recoverMessageInput(message, history)
}

// Delivery is inferred from standard history. Neither method changes events or
// requires the SessionService implementation to know the business schema.
func recoverConfirmationInput(inputs []*schema.WaitInput, history session.Session) (*genai.Content, error) {
	if !waitInputsReady(inputs) {
		return nil, errors.New("incomplete submitted confirmation input")
	}
	delivered := make(map[string]bool, len(inputs))
	wanted := make(map[string]string, len(inputs))
	for _, input := range inputs {
		wanted[input.ID] = input.Content
	}
	if history != nil {
		for event := range history.Events().All() {
			if event == nil || event.Author != "user" || event.Content == nil || event.Content.Role != genai.RoleUser {
				continue
			}
			for _, part := range event.Content.Parts {
				if part == nil || part.FunctionResponse == nil || part.FunctionResponse.Name != toolconfirmation.FunctionCallName {
					continue
				}
				response := part.FunctionResponse
				content, ok := wanted[response.ID]
				if !ok {
					continue
				}
				if response.Response["payload"] != content || response.Response["confirmed"] != true {
					return nil, fmt.Errorf("confirmation history conflicts with input %q", response.ID)
				}
				delivered[response.ID] = true
			}
		}
	}
	missing := make([]*schema.WaitInput, 0, len(inputs))
	for _, input := range inputs {
		if !delivered[input.ID] {
			missing = append(missing, input)
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}
	return buildInputRequiredMessage(missing), nil
}

func recoverMessageInput(message *schema.Message, history session.Session) (*genai.Content, error) {
	if message == nil || message.Content == "" {
		return nil, errors.New("missing user message for running session")
	}
	start := 0
	var events session.Events
	if history != nil {
		events = history.Events()
	}
	if message.ADKEventCursor != nil && *message.ADKEventCursor != "" {
		found := false
		if events != nil {
			for i := 0; i < events.Len(); i++ {
				if event := events.At(i); event != nil && event.ID == *message.ADKEventCursor {
					start, found = i+1, true
					break
				}
			}
		}
		if !found {
			return nil, errors.New("input history cursor is no longer available")
		}
	}
	if events != nil {
		for i := start; i < events.Len(); i++ {
			event := events.At(i)
			if event == nil || event.Author != "user" || event.Content == nil || event.Content.Role != genai.RoleUser {
				continue
			}
			// Older records lack a cursor; retain timestamp-based recovery for
			// them. New records use an exact event boundary, even for equal text.
			if message.ADKEventCursor == nil && event.Timestamp.Before(message.CreatedAt) {
				continue
			}
			parts := event.Content.Parts
			if len(parts) == 1 && parts[0] != nil && parts[0].Text == message.Content && parts[0].FunctionResponse == nil {
				return nil, nil
			}
		}
	}
	return genai.NewContentFromText(message.Content, genai.RoleUser), nil
}
