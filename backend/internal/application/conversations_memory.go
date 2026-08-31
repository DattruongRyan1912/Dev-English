package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

var _ ConversationRepository = (*MemoryConversationRepository)(nil)

type MemoryConversationRepository struct {
	mu            sync.RWMutex
	conversations map[conversationKey]Conversation
	sequence      uint64
}

type conversationKey struct {
	workspaceID    string
	userID         string
	conversationID string
}

func NewMemoryConversationRepository() *MemoryConversationRepository {
	return &MemoryConversationRepository{conversations: make(map[conversationKey]Conversation)}
}

func (r *MemoryConversationRepository) Create(_ context.Context, scope work.Scope, title string, contexts ...assistant.ContextRef) (Conversation, error) {
	if err := scope.Validate(); err != nil {
		return Conversation{}, err
	}
	contextRef, err := normalizeConversationContext(contexts)
	if err != nil {
		return Conversation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sequence++
	now := time.Now().UTC()
	conversation := Conversation{ID: fmt.Sprintf("conversation-%d", r.sequence), WorkspaceID: scope.WorkspaceID, UserID: scope.UserID, Title: title, Status: "active", Context: contextRefPtr(contextRef), CreatedAt: now, UpdatedAt: now, Messages: []ConversationMessage{}}
	r.conversations[conversationKey{scope.WorkspaceID, scope.UserID, conversation.ID}] = cloneConversation(conversation)
	return conversation, nil
}

func (r *MemoryConversationRepository) List(_ context.Context, scope work.Scope, limit int) ([]ConversationSummary, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ConversationSummary, 0)
	for key, conversation := range r.conversations {
		if key.workspaceID != scope.WorkspaceID || key.userID != scope.UserID {
			continue
		}
		items = append(items, conversationSummary(conversation))
	}
	sortConversationSummaries(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemoryConversationRepository) Get(_ context.Context, scope work.Scope, id string) (Conversation, error) {
	if err := scope.Validate(); err != nil {
		return Conversation{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id == "latest" {
		var latest Conversation
		found := false
		for key, item := range r.conversations {
			if key.workspaceID != scope.WorkspaceID || key.userID != scope.UserID {
				continue
			}
			if !found || item.UpdatedAt.After(latest.UpdatedAt) {
				latest = item
				found = true
			}
		}
		if !found {
			return Conversation{}, ErrConversationNotFound
		}
		return cloneConversation(latest), nil
	}
	conversation, ok := r.conversations[conversationKey{scope.WorkspaceID, scope.UserID, id}]
	if !ok {
		return Conversation{}, ErrConversationNotFound
	}
	return cloneConversation(conversation), nil
}

func (r *MemoryConversationRepository) Append(_ context.Context, scope work.Scope, conversationID, userMessage string, response assistant.AssistantResponse) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := conversationKey{scope.WorkspaceID, scope.UserID, conversationID}
	conversation, ok := r.conversations[key]
	if !ok {
		return ErrConversationNotFound
	}
	r.sequence++
	now := time.Now().UTC()
	responseCopy := cloneAssistantResponse(response)
	conversation.Messages = append(conversation.Messages,
		ConversationMessage{ID: fmt.Sprintf("message-%d", r.sequence), Role: "user", Content: userMessage, CreatedAt: now},
		ConversationMessage{ID: fmt.Sprintf("message-%d", r.sequence+1), Role: "assistant", Content: response.Answer, Response: &responseCopy, CreatedAt: now.Add(time.Nanosecond)},
	)
	r.sequence++
	conversation.UpdatedAt = now
	r.conversations[key] = cloneConversation(conversation)
	return nil
}

func cloneConversation(value Conversation) Conversation {
	if value.Context != nil {
		contextRef := *value.Context
		value.Context = &contextRef
	}
	value.Messages = append([]ConversationMessage(nil), value.Messages...)
	for index := range value.Messages {
		if value.Messages[index].Response != nil {
			copy := cloneAssistantResponse(*value.Messages[index].Response)
			value.Messages[index].Response = &copy
		}
	}
	return value
}

func conversationSummary(value Conversation) ConversationSummary {
	var contextRef *assistant.ContextRef
	if value.Context != nil {
		copy := *value.Context
		contextRef = &copy
	}
	return ConversationSummary{
		ID:           value.ID,
		WorkspaceID:  value.WorkspaceID,
		UserID:       value.UserID,
		Title:        value.Title,
		Status:       value.Status,
		Context:      contextRef,
		MessageCount: len(value.Messages),
		CreatedAt:    value.CreatedAt,
		UpdatedAt:    value.UpdatedAt,
	}
}

func sortConversationSummaries(items []ConversationSummary) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func normalizeConversationContext(contexts []assistant.ContextRef) (assistant.ContextRef, error) {
	if len(contexts) > 1 {
		return assistant.ContextRef{}, fmt.Errorf("%w: conversation context must contain at most one reference", assistant.ErrInvalidInput)
	}
	if len(contexts) == 0 {
		return assistant.ContextRef{}, nil
	}
	return contexts[0].Normalize()
}

func contextRefPtr(contextRef assistant.ContextRef) *assistant.ContextRef {
	if contextRef.IsZero() {
		return nil
	}
	copy := contextRef
	return &copy
}

func cloneAssistantResponse(value assistant.AssistantResponse) assistant.AssistantResponse {
	result := value
	result.Evidence = append([]assistant.Citation(nil), value.Evidence...)
	result.Unknowns = append([]string(nil), value.Unknowns...)
	result.StaleSources = append([]string(nil), value.StaleSources...)
	result.SuggestedActions = append([]assistant.SuggestedAction(nil), value.SuggestedActions...)
	result.ActionReceipts = append([]assistant.ActionReceipt(nil), value.ActionReceipts...)
	return result
}

func encodeAssistantResponse(value assistant.AssistantResponse) ([]byte, error) {
	return json.Marshal(value)
}

func sortConversations(items []Conversation) {
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
}
