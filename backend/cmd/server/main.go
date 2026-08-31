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

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/httpapi"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/integrations"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learningoverlay"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/mcp"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/platform"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/secrets"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	environment := runtimeEnvironment()
	moduleManifest, manifestErr := platform.ParseManifest(os.Getenv("DEVENGLISH_MODULES"))
	if manifestErr != nil {
		logger.Error("invalid module manifest", "error", manifestErr)
		os.Exit(1)
	}
	production := environment == "production"
	authManager := auth.NewFromEnv()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	allowedOrigins := parseAllowedOrigins(os.Getenv("DEVENGLISH_ALLOWED_ORIGINS"))
	loginSecret := strings.TrimSpace(os.Getenv("DEVENGLISH_LOGIN_SECRET"))
	if err := validateRuntimeConfig(environment, authManager, databaseURL, allowedOrigins, loginSecret); err != nil {
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
	deepseek := ai.NewDeepSeekFromEnv()
	secretBox, secretErr := secrets.NewFromEnv()
	if secretErr != nil && production {
		logger.Error("DeepSeek secret encryption is not configured", "error", secretErr)
		os.Exit(1)
	}
	provider := ai.FallbackProvider{Primary: deepseek, Fallback: ai.DeterministicProvider{}, AllowFallback: !production}
	stt := ai.NewGroqSTTFromEnv()
	azure := ai.NewAzureSpeechFromEnv()
	github := integrations.NewGitHubFromEnv()
	service := learning.NewService(repository, provider, learning.SpeechDependencies{STT: stt, TTS: azure, Pronunciation: azure, GitHub: github})
	service.DeepSeekSecrets = learning.NewDeepSeekSecretManager(repository, deepseek, secretBox)
	if err := service.DeepSeekSecrets.Load(context.Background()); err != nil {
		logger.Error("DeepSeek secret load failed", "error", err)
		if production {
			os.Exit(1)
		}
	}
	if settings, err := repository.Settings(context.Background()); err == nil {
		if settings.FastModel != "" && settings.SmartModel != "" {
			if err := deepseek.SetModels(settings.FastModel, settings.SmartModel); err != nil {
				logger.Warn("stored DeepSeek model settings are invalid", "error", err)
			}
		}
		settings.DeepSeekConfigured = provider.Configured()
		if settings.DeepSeekConfigured && settings.DeepSeekStatus == "not_configured" {
			settings.DeepSeekStatus = learning.DeepSeekStatusConnected
		}
		if !settings.DeepSeekConfigured {
			settings.DeepSeekStatus = learning.DeepSeekStatusNotConfigured
		}
		settings.SpeechConfigured = stt.Configured() || azure.Configured()
		settings.PronunciationOn = azure.PronunciationConfigured()
		if err := repository.SaveSettings(context.Background(), settings); err != nil {
			logger.Warn("could not update provider settings", "error", err)
		}
	}
	var productApp *application.App
	var appErr error
	if postgres != nil {
		productApp, appErr = application.NewPostgres(postgres.Pool)
	} else {
		productApp, appErr = application.NewMemory()
	}
	if appErr != nil {
		logger.Error("product application initialization failed", "error", appErr)
		os.Exit(1)
	}
	// Development/test use a seeded user so the local shell can start without a
	// login round-trip. Production must not invent an owner before the first
	// authenticated login; authLogin ensures the user and workspace lazily.
	if !production {
		if appErr := productApp.EnsureDefaultScope(store.WithUser(context.Background(), "user-1")); appErr != nil {
			logger.Error("product workspace initialization failed", "error", appErr)
			os.Exit(1)
		}
	}
	assistantGenerator := application.NewDeepSeekAssistantGenerator(deepseek, repository, environment == "development" || environment == "test")
	assistantGenerator.StrictUsage = production
	if err := productApp.SetAssistantGenerator(assistantGenerator); err != nil {
		logger.Error("assistant generator initialization failed", "error", err)
		os.Exit(1)
	}
	var embedder knowledge.EmbeddingProvider
	if configuredEmbedder := knowledge.NewHTTPEmbeddingProviderFromEnv(); configuredEmbedder != nil {
		embedder = configuredEmbedder
		productApp.SetKnowledgeEmbedder(configuredEmbedder)
	}
	var learningOverlayRepository learningoverlay.Repository
	if postgres != nil {
		repository, err := learningoverlay.NewPostgresRepository(postgres.Pool)
		if err != nil {
			logger.Error("learning overlay initialization failed", "error", err)
			os.Exit(1)
		}
		learningOverlayRepository = repository
	} else {
		learningOverlayRepository = learningoverlay.NewMemoryRepository()
	}
	learningOverlayService, err := learningoverlay.NewService(learningOverlayRepository)
	if err != nil {
		logger.Error("learning overlay service initialization failed", "error", err)
		os.Exit(1)
	}
	var challengeIssuer connectors.SafeWriteChallengeIssuer
	var challengeReader connectors.SafeWriteChallengeReader
	var challengeStore connectors.SafeWriteChallengeStore
	var receiptStore connectors.SafeWriteReceiptStore
	if postgres != nil {
		postgresChallenges, err := connectors.NewPostgresSafeWriteChallengeStore(postgres.Pool)
		if err != nil {
			logger.Error("safe-write challenge store initialization failed", "error", err)
			os.Exit(1)
		}
		postgresReceipts, err := connectors.NewPostgresSafeWriteReceiptStore(postgres.Pool)
		if err != nil {
			logger.Error("safe-write receipt store initialization failed", "error", err)
			os.Exit(1)
		}
		challengeIssuer, challengeReader, challengeStore, receiptStore = postgresChallenges, postgresChallenges, postgresChallenges, postgresReceipts
	} else {
		memoryChallenges := connectors.NewMemorySafeWriteChallengeStore()
		challengeIssuer, challengeReader, challengeStore = memoryChallenges, memoryChallenges, memoryChallenges
		receiptStore = connectors.NewMemorySafeWriteReceiptStore()
	}
	guardedGitHub, err := connectors.NewGitHubSafeWriteService(github, challengeStore, receiptStore)
	if err != nil {
		logger.Error("safe-write service initialization failed", "error", err)
		os.Exit(1)
	}
	actionService, err := actions.NewServiceWithWork(guardedGitHub, productApp.Work, challengeIssuer, challengeReader)
	if err != nil {
		logger.Error("action service initialization failed", "error", err)
		os.Exit(1)
	}
	var tokenPersistence mcp.TokenPersistence
	if postgres != nil {
		tokenPersistence, err = mcp.NewPostgresTokenPersistence(postgres.Pool)
		if err != nil {
			logger.Error("MCP token persistence initialization failed", "error", err)
			os.Exit(1)
		}
	}
	mcpTokenStore := mcp.NewTokenStore(mcp.WithTokenPersistence(tokenPersistence))
	mcpApplication, err := mcp.NewApplication(mcp.ApplicationServices{
		Assistant: productApp.Assistant,
		Knowledge: productApp.Knowledge,
		Work:      productApp.Work,
		Product:   productApp,
		Actions:   actionService,
	})
	if err != nil {
		logger.Error("MCP application initialization failed", "error", err)
		os.Exit(1)
	}
	mcpRegistry := mcp.NewRegistry()
	if err := mcpApplication.Register(mcpRegistry); err != nil {
		logger.Error("MCP registry initialization failed", "error", err)
		os.Exit(1)
	}
	api := httpapi.NewServer(service, logger, authManager)
	api.Modules = moduleManifest
	api.Application = productApp
	api.Actions = actionService
	api.LearningOverlay = learningOverlayService
	api.MCPTokenStore = mcpTokenStore
	api.MCP = mcp.NewHandler(mcpRegistry, mcpTokenStore)
	if postgres != nil {
		driveReader := integrations.NewGoogleDriveFromEnv()
		api.DriveSync = func(workspaceID string) (*connectors.DriveSyncService, error) {
			sink, err := connectors.NewPostgresDriveRevisionStoreWithEmbedder(postgres.Pool, workspaceID, embedder)
			if err != nil {
				return nil, err
			}
			return connectors.NewDriveSyncService(driveReader, sink)
		}
		api.GitHubSync = func(workspaceID string) (*connectors.GitHubImportService, error) {
			sink, err := connectors.NewPostgresGitHubRevisionStoreWithEmbedder(postgres.Pool, workspaceID, embedder)
			if err != nil {
				return nil, err
			}
			return connectors.NewGitHubImportService(github, sink)
		}
	}
	api.StrictAuth = production
	api.AllowedOrigins = allowedOrigins
	api.LoginSecret = loginSecret
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

func validateRuntimeConfig(environment string, authManager *auth.Manager, databaseURL string, allowedOrigins []string, loginSecret string) error {
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
	if len(loginSecret) < 16 {
		return fmt.Errorf("DEVENGLISH_LOGIN_SECRET must be at least 16 characters in production")
	}
	if len(strings.TrimSpace(os.Getenv("DEVENGLISH_SECRET_ENCRYPTION_KEY"))) < 32 {
		return fmt.Errorf("DEVENGLISH_SECRET_ENCRYPTION_KEY must be at least 32 characters in production")
	}
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required in production")
	}
	if len(allowedOrigins) == 0 {
		return fmt.Errorf("DEVENGLISH_ALLOWED_ORIGINS must contain at least one origin in production")
	}
	for _, origin := range allowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.Scheme != "https" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.User != nil || origin == "*" {
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
