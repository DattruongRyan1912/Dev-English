package learning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/secrets"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDeepSeekSecretManagerEncryptsPersistsReloadsAndRemovesKey(t *testing.T) {
	const apiKey = "sk-deepseek-test-key-1234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Fatalf("provider key leaked incorrectly or was not applied: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "fast"}, {"id": "smart"}}})
	}))
	defer server.Close()

	memory := store.NewSeeded(time.Now().UTC())
	provider := &ai.DeepSeekProvider{BaseURL: server.URL, FastModel: "fast", SmartModel: "smart", Client: server.Client()}
	box, err := secrets.New(strings.Repeat("deployment-key", 3))
	if err != nil {
		t.Fatal(err)
	}
	manager := NewDeepSeekSecretManager(memory, provider, box)
	settings, check, err := manager.Set(context.Background(), apiKey)
	if err != nil {
		t.Fatal(err)
	}
	if settings.DeepSeekStatus != DeepSeekStatusConnected || check.Status != "healthy" {
		t.Fatalf("unexpected configured state: settings=%+v check=%+v", settings, check)
	}
	stored, err := memory.DeepSeekSecret(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored.Ciphertext), apiKey) {
		t.Fatal("stored ciphertext contains the raw provider key")
	}

	reloaded := &ai.DeepSeekProvider{BaseURL: server.URL, FastModel: "fast", SmartModel: "smart", Client: server.Client()}
	reloadManager := NewDeepSeekSecretManager(memory, reloaded, box)
	if err := reloadManager.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reloaded.Configured() {
		t.Fatal("reloaded provider was not configured from the encrypted secret")
	}

	settings, err = manager.Remove(context.Background())
	if err != nil || settings.DeepSeekStatus != DeepSeekStatusNotConfigured || provider.Configured() {
		t.Fatalf("remove did not clear provider state: settings=%+v err=%v configured=%v", settings, err, provider.Configured())
	}
}
