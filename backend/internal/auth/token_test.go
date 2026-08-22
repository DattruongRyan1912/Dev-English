package auth

import (
	"strings"
	"testing"
	"time"
)

func TestManagerIssuesAndValidatesSignedToken(t *testing.T) {
	manager := &Manager{secret: []byte(strings.Repeat("x", 32)), TTL: time.Hour}
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	token, claims, err := manager.Issue("user-42", now)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := manager.Parse(token, now.Add(time.Minute))
	if err != nil || parsed.Subject != claims.Subject {
		t.Fatalf("unexpected claims: %+v, err=%v", parsed, err)
	}
	if _, err := manager.Parse(token+"x", now); err == nil {
		t.Fatal("tampered token must be rejected")
	}
}

func TestManagerRejectsExpiredToken(t *testing.T) {
	manager := &Manager{secret: []byte(strings.Repeat("x", 32)), TTL: time.Minute}
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	token, _, err := manager.Issue("user-42", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(token, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired token must be rejected")
	}
}
