package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/secrets"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

const (
	DeepSeekStatusNotConfigured = "not_configured"
	DeepSeekStatusConnected     = "connected"
	DeepSeekStatusInvalid       = "invalid"
)

// DeepSeekSecretManager keeps the provider hot-reloadable while ensuring the
// repository only receives authenticated ciphertext and a nonce.
type DeepSeekSecretManager struct {
	Store    store.Repository
	Provider *ai.DeepSeekProvider
	Box      *secrets.Box
}

func NewDeepSeekSecretManager(repository store.Repository, provider *ai.DeepSeekProvider, box *secrets.Box) *DeepSeekSecretManager {
	return &DeepSeekSecretManager{Store: repository, Provider: provider, Box: box}
}

func (m *DeepSeekSecretManager) Load(ctx context.Context) error {
	if m == nil || m.Store == nil || m.Provider == nil {
		return nil
	}
	secret, err := m.Store.DeepSeekSecret(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load DeepSeek secret: %w", err)
	}
	if m.Box == nil {
		return errors.New("DeepSeek secret exists but encryption is not configured")
	}
	key, err := m.Box.Open(secret.Ciphertext, secret.Nonce)
	if err != nil {
		return fmt.Errorf("decrypt DeepSeek secret: %w", err)
	}
	m.Provider.SetAPIKey(key)
	return nil
}

func (m *DeepSeekSecretManager) Set(ctx context.Context, rawKey string) (domain.Settings, domain.ProviderCheck, error) {
	key := strings.TrimSpace(rawKey)
	if err := validateProviderKey(key); err != nil {
		return domain.Settings{}, domain.ProviderCheck{}, err
	}
	if m == nil || m.Store == nil || m.Provider == nil || m.Box == nil {
		return domain.Settings{}, domain.ProviderCheck{}, errors.New("DeepSeek secret management is not configured")
	}
	ciphertext, nonce, err := m.Box.Seal(key)
	if err != nil {
		return domain.Settings{}, domain.ProviderCheck{}, err
	}
	if err := m.Store.SaveDeepSeekSecret(ctx, store.EncryptedSecret{Ciphertext: ciphertext, Nonce: nonce}); err != nil {
		return domain.Settings{}, domain.ProviderCheck{}, fmt.Errorf("save DeepSeek secret: %w", err)
	}
	m.Provider.SetAPIKey(key)
	return m.Test(ctx)
}

func (m *DeepSeekSecretManager) Remove(ctx context.Context) (domain.Settings, error) {
	if m == nil || m.Store == nil || m.Provider == nil {
		return domain.Settings{}, errors.New("DeepSeek secret management is not configured")
	}
	if err := m.Store.DeleteDeepSeekSecret(ctx); err != nil {
		return domain.Settings{}, fmt.Errorf("remove DeepSeek secret: %w", err)
	}
	m.Provider.ClearAPIKey()
	settings, err := m.Store.Settings(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	settings.DeepSeekConfigured = false
	settings.DeepSeekStatus = DeepSeekStatusNotConfigured
	if err := m.Store.SaveSettings(ctx, settings); err != nil {
		return domain.Settings{}, err
	}
	return settings, nil
}

func (m *DeepSeekSecretManager) Test(ctx context.Context) (domain.Settings, domain.ProviderCheck, error) {
	if m == nil || m.Store == nil || m.Provider == nil {
		return domain.Settings{}, domain.ProviderCheck{}, errors.New("DeepSeek secret management is not configured")
	}
	settings, err := m.Store.Settings(ctx)
	if err != nil {
		return domain.Settings{}, domain.ProviderCheck{}, err
	}
	check := m.Provider.ProbeCapability(ctx, "text_generation")
	check.Provider = "DeepSeek"
	if !check.Configured {
		check.Status = "not_configured"
		settings.DeepSeekConfigured = false
		settings.DeepSeekStatus = DeepSeekStatusNotConfigured
	} else {
		settings.DeepSeekConfigured = true
		if !check.Healthy {
			check.Status = "unhealthy"
			settings.DeepSeekStatus = DeepSeekStatusInvalid
		} else {
			check.Status = "healthy"
			settings.DeepSeekStatus = DeepSeekStatusConnected
		}
	}
	if saveErr := m.Store.SaveSettings(ctx, settings); saveErr != nil {
		return domain.Settings{}, domain.ProviderCheck{}, saveErr
	}
	return settings, check, nil
}

func validateProviderKey(value string) error {
	if value == "" {
		return errors.New("DeepSeek API key is required")
	}
	if len(value) < 16 || len(value) > 512 || strings.ContainsAny(value, "\r\n\t ") {
		return errors.New("DeepSeek API key has an invalid format")
	}
	return nil
}
