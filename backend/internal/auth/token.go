package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid authentication token")

type Claims struct {
	Subject   string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
}

type Manager struct {
	secret       []byte
	TTL          time.Duration
	BootstrapKey string
}

func New(secret string) *Manager {
	return &Manager{secret: []byte(strings.TrimSpace(secret)), TTL: 7 * 24 * time.Hour}
}

func NewFromEnv() *Manager {
	secret := strings.TrimSpace(os.Getenv("DEVENGLISH_AUTH_SECRET"))
	if secret == "" {
		return &Manager{}
	}
	return &Manager{secret: []byte(secret), TTL: 7 * 24 * time.Hour, BootstrapKey: os.Getenv("DEVENGLISH_BOOTSTRAP_KEY")}
}

func (m *Manager) Enabled() bool { return m != nil && len(m.secret) >= 32 }

func (m *Manager) Issue(subject string, now time.Time) (string, Claims, error) {
	if !m.Enabled() {
		return "", Claims{}, errors.New("authentication is disabled")
	}
	if strings.TrimSpace(subject) == "" {
		return "", Claims{}, errors.New("subject is required")
	}
	if m.TTL <= 0 {
		m.TTL = 7 * 24 * time.Hour
	}
	claims := Claims{Subject: subject, ExpiresAt: now.Add(m.TTL).Unix()}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", Claims{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encoded)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), claims, nil
}

func (m *Manager) Parse(token string, now time.Time) (Claims, error) {
	if !m.Enabled() {
		return Claims{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return Claims{}, ErrInvalidToken
	}
	expected := m.sign(parts[0])
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Subject == "" || claims.ExpiresAt <= now.Unix() {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) BootstrapAllowed(value string) bool {
	if m == nil || m.BootstrapKey == "" {
		return true
	}
	return hmac.Equal([]byte(m.BootstrapKey), []byte(value))
}

func (m *Manager) sign(payload string) []byte {
	hash := hmac.New(sha256.New, m.secret)
	_, _ = hash.Write([]byte(payload))
	return hash.Sum(nil)
}
