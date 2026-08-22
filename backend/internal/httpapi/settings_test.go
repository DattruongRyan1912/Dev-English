package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/secrets"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func TestDeepSeekSettingsNeverReturnRawKeyAndSupportLifecycle(t *testing.T) {
	const apiKey = "sk-deepseek-settings-test-1234567890"
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Fatalf("unexpected provider authorization: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"fast"},{"id":"smart"}]}`)
	}))
	defer providerServer.Close()

	memory := store.NewSeeded(time.Now().UTC())
	provider := &ai.DeepSeekProvider{BaseURL: providerServer.URL, FastModel: "fast", SmartModel: "smart", Client: providerServer.Client()}
	box, err := secrets.New(strings.Repeat("deployment-key", 3))
	if err != nil {
		t.Fatal(err)
	}
	service := learning.NewService(memory, ai.DeterministicProvider{})
	service.DeepSeekSecrets = learning.NewDeepSeekSecretManager(memory, provider, box)
	handler := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	set := httptest.NewRecorder()
	setRequest := httptest.NewRequest(http.MethodPut, "/api/v1/settings/deepseek", strings.NewReader(`{"apiKey":"`+apiKey+`"}`))
	setRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(set, setRequest)
	if set.Code != http.StatusOK || strings.Contains(set.Body.String(), apiKey) || !strings.Contains(set.Body.String(), `"deepSeekStatus":"connected"`) {
		t.Fatalf("unsafe or unsuccessful key set response: %d %s", set.Code, set.Body.String())
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	if get.Code != http.StatusOK || strings.Contains(get.Body.String(), apiKey) || !strings.Contains(get.Body.String(), `"deepSeekConfigured":true`) {
		t.Fatalf("settings GET exposed key or missed metadata: %d %s", get.Code, get.Body.String())
	}

	test := httptest.NewRecorder()
	handler.ServeHTTP(test, httptest.NewRequest(http.MethodPost, "/api/v1/settings/deepseek/test", nil))
	if test.Code != http.StatusOK || !strings.Contains(test.Body.String(), `"status":"healthy"`) {
		t.Fatalf("DeepSeek test failed: %d %s", test.Code, test.Body.String())
	}

	invalidModel := httptest.NewRecorder()
	invalidModelRequest := httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(`{"aiProvider":"deepseek","fastModel":"bad model","smartModel":"smart","pronunciationOn":false,"monthlyBudgetVnd":150000}`))
	invalidModelRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(invalidModel, invalidModelRequest)
	if invalidModel.Code != http.StatusBadRequest {
		t.Fatalf("invalid model returned %d: %s", invalidModel.Code, invalidModel.Body.String())
	}

	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, "/api/v1/settings/deepseek", nil))
	if remove.Code != http.StatusOK || !strings.Contains(remove.Body.String(), `"deepSeekStatus":"not_configured"`) {
		t.Fatalf("DeepSeek remove failed: %d %s", remove.Code, remove.Body.String())
	}
	var settings map[string]any
	if err := json.Unmarshal(remove.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["deepSeekConfigured"] == true {
		t.Fatal("removed DeepSeek key still reports configured")
	}
}
