package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/ownforge/ownforge/pkg/logger"
	commonRedis "github.com/ownforge/ownforge/pkg/redis"
	"github.com/ownforge/ownforge/services/notes/internal/event"
	"github.com/ownforge/ownforge/services/notes/internal/platform/config"
	"github.com/ownforge/ownforge/services/notes/internal/platform/database"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	"github.com/ownforge/ownforge/services/notes/internal/platform/llm"
	"github.com/ownforge/ownforge/services/notes/internal/platform/lock"
	ai "github.com/ownforge/ownforge/services/notes/internal/service/ai"
	aicallogstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/aicallog"
	aimetadatastore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/aimetadata"
	snippetstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/snippet"
	tagstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/tag"
	"go.uber.org/zap"
)

func main() {
	// ── Load environment variables ──
	_ = godotenv.Load()

	// ── Logging ──
	log := logger.NewLogger("ai-worker")
	defer log.Sync()

	// ── Config ──
	cfg := config.LoadConfig()
	if err := validateAIConfig(cfg.AI); err != nil {
		log.Fatal("invalid AI worker configuration", zap.Error(err))
	}

	log.Info("starting AI worker",
		zap.String("provider", cfg.AI.Provider),
		zap.String("model", cfg.AI.EnrichModel),
		zap.Int("worker_count", workerCount(cfg.AI)),
	)

	// ── Database ──
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	defer entClient.Close()

	// ── Redis ──
	redisClient := commonRedis.Init(cfg.Redis, log)
	defer redisClient.Close()

	// ── Repositories ──
	snippetRepo := snippetstore.New(entClient)
	tagRepo := tagstore.New(entClient)
	aiMetadataRepo := aimetadatastore.New(entClient)
	callLogRepo := aicallogstore.New(entClient)

	// ── ID generator (call log records need allocated IDs) ──
	idgenClient, err := idgen.New(cfg.IDGenerator.Addr)
	if err != nil {
		log.Fatal("failed to connect to id-generator", zap.String("addr", cfg.IDGenerator.Addr), zap.Error(err))
	}
	defer idgenClient.Close()

	// ── LLM Client ──
	llmClient := newLLMClient(cfg.AI)

	// ── Distributed lock (prevents concurrent workers from processing the same snippet twice) ──
	locker := lock.NewRedisLocker(redisClient)

	// ── EnrichService ──
	enrichSvc := ai.NewEnrichService(
		snippetRepo,
		tagRepo,
		aiMetadataRepo,
		callLogRepo,
		idgenClient,
		llmClient,
		locker,
		cfg.AI.Provider,
		cfg.AI.EnrichModel,
		cfg.AI.MaxTokens,
		cfg.AI.MinContentLen,
		log,
	)

	// ── Redis Stream Subscriber ──
	subscriber := event.NewRedisStreamSubscriber(redisClient, log)
	defer subscriber.Close()

	// ── Start workers ──
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := workerCount(cfg.AI)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			consumerName := fmt.Sprintf("worker-%d", workerID)

			log.Info("worker started", zap.Int("worker_id", workerID))

			err := subscriber.Subscribe(ctx, event.TopicSnippetSaved, "ai-enricher", consumerName,
				func(ctx context.Context, data []byte) error {
					var payload event.SnippetSavedPayload
					if err := json.Unmarshal(data, &payload); err != nil {
						log.Error("failed to deserialize event", zap.Error(err), zap.ByteString("data", data))
						return nil // Do not retry malformed messages
					}

					log.Info("starting snippet enrichment",
						zap.Int64("snippet_id", payload.SnippetID),
						zap.String("action", payload.Action),
					)

					_, err := enrichSvc.Enrich(ctx, payload.SnippetID)
					if err != nil {
						log.Error("snippet enrichment failed",
							zap.Int64("snippet_id", payload.SnippetID),
							zap.Error(err),
						)
						return err // Return an error -> do not ACK -> leave it for retry
					}
					return nil
				},
			)
			if err != nil && err != context.Canceled {
				log.Error("worker exited unexpectedly", zap.Int("worker_id", workerID), zap.Error(err))
			}
		}(i)
	}

	// ── Graceful shutdown ──
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("received shutdown signal, stopping worker...", zap.String("signal", sig.String()))

	cancel()
	wg.Wait()
	log.Info("AI worker stopped")
}

// newLLMClient creates the matching LLM client from config.
func newLLMClient(cfg config.AIConfig) llm.Client {
	switch cfg.Provider {
	case "anthropic":
		return llm.NewAnthropicClient(cfg.BaseURL, cfg.APIKey)
	default: // "openai", "ollama", "qwen", or any OpenAI-compatible endpoint
		return llm.NewOpenAICompatClient(cfg.BaseURL, cfg.APIKey)
	}
}

// workerCount returns the worker concurrency, defaulting to 4.
func workerCount(cfg config.AIConfig) int {
	if cfg.WorkerCount > 0 {
		return cfg.WorkerCount
	}
	return 4
}

func validateAIConfig(cfg config.AIConfig) error {
	if cfg.EnrichModel == "" {
		return fmt.Errorf("AI_ENRICH_MODEL cannot be empty")
	}
	return nil
}
