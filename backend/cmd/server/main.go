package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	port := envInt("PORT", 8080)
	memory := store.NewSeeded(time.Now().UTC())
	var repository store.Repository = memory
	var postgres *store.PostgresStore
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		var err error
		postgres, err = store.NewPostgres(context.Background(), databaseURL)
		if err != nil {
			logger.Error("postgres initialization failed", "error", err)
			os.Exit(1)
		}
		if err := postgres.EnsureLocalSeed(store.WithUser(context.Background(), "user-1")); err != nil {
			logger.Error("postgres seed failed", "error", err)
			postgres.Close()
			os.Exit(1)
		}
		repository = postgres
	}
	if postgres != nil {
		defer postgres.Close()
	}
	provider := ai.FallbackProvider{Primary: ai.NewDeepSeekFromEnv(), Fallback: ai.DeterministicProvider{}}
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
	authManager := auth.NewFromEnv()
	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: httpapi.NewServer(service, logger, authManager).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second}

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

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
