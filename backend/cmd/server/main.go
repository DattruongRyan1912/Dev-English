package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/httpapi"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/integrations"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	environment := runtimeEnvironment()
	production := environment == "production"
	authManager := auth.NewFromEnv()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	allowedOrigins := parseAllowedOrigins(os.Getenv("DEVENGLISH_ALLOWED_ORIGINS"))
	if err := validateRuntimeConfig(environment, authManager, databaseURL, allowedOrigins); err != nil {
		logger.Error("invalid runtime configuration", "error", err, "environment", environment)
		os.Exit(1)
	}
	port := envInt("PORT", 8080)
	memory := store.NewSeeded(time.Now().UTC())
	var repository store.Repository = memory
	var postgres *store.PostgresStore
	if databaseURL != "" {
		var err error
		postgres, err = store.NewPostgres(context.Background(), databaseURL)
		if err != nil {
			logger.Error("postgres initialization failed", "error", err)
			os.Exit(1)
		}
		if !production {
			if err := postgres.EnsureLocalSeed(store.WithUser(context.Background(), "user-1")); err != nil {
				logger.Error("postgres seed failed", "error", err)
				postgres.Close()
				os.Exit(1)
			}
		}
		repository = postgres
	}
	if postgres != nil {
		defer postgres.Close()
	}
	provider := ai.FallbackProvider{Primary: ai.NewDeepSeekFromEnv(), Fallback: ai.DeterministicProvider{}, AllowFallback: !production}
	stt := ai.NewGroqSTTFromEnv()
	azure := ai.NewAzureSpeechFromEnv()
	github := integrations.NewGitHubFromEnv()
	if settings, err := repository.Settings(context.Background()); err == nil {
		settings.DeepSeekConfigured = provider.Configured()
		settings.SpeechConfigured = stt.Configured() || azure.Configured()
		settings.PronunciationOn = azure.Configured()
		if err := repository.SaveSettings(context.Background(), settings); err != nil {
			logger.Warn("could not update provider settings", "error", err)
		}
	}
	service := learning.NewService(repository, provider, learning.SpeechDependencies{STT: stt, TTS: azure, Pronunciation: azure, GitHub: github})
	api := httpapi.NewServer(service, logger, authManager)
	api.StrictAuth = production
	api.AllowedOrigins = allowedOrigins
	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		logger.Info("devenglish backend started", "port", port, "provider", provider.Name(), "provider_configured", provider.Configured())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func runtimeEnvironment() string {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("DEVENGLISH_ENV")))
	if environment == "" {
		return "development"
	}
	return environment
}

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func validateRuntimeConfig(environment string, authManager *auth.Manager, databaseURL string, allowedOrigins []string) error {
	switch environment {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("DEVENGLISH_ENV must be development, test, staging, or production")
	}
	if environment != "production" {
		return nil
	}
	if authManager == nil || !authManager.Enabled() {
		return fmt.Errorf("DEVENGLISH_AUTH_SECRET must be at least 32 characters in production")
	}
	if strings.TrimSpace(os.Getenv("DEVENGLISH_BOOTSTRAP_KEY")) == "" {
		return fmt.Errorf("DEVENGLISH_BOOTSTRAP_KEY is required in production")
	}
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required in production")
	}
	if len(allowedOrigins) == 0 {
		return fmt.Errorf("DEVENGLISH_ALLOWED_ORIGINS must contain at least one origin in production")
	}
	for _, origin := range allowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Path != "" || origin == "*" {
			return fmt.Errorf("invalid production CORS origin %q", origin)
		}
	}
	return nil
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
