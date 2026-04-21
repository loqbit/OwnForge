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

	"github.com/luckysxx/common/logger"
	commonRedis "github.com/luckysxx/common/redis"
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
	// ── 加载环境变量 ──
	_ = godotenv.Load()

	// ── 日志 ──
	log := logger.NewLogger("ai-worker")
	defer log.Sync()

	// ── 配置 ──
	cfg := config.LoadConfig()
	if err := validateAIConfig(cfg.AI); err != nil {
		log.Fatal("AI Worker 配置无效", zap.Error(err))
	}

	log.Info("AI Worker 启动中",
		zap.String("provider", cfg.AI.Provider),
		zap.String("model", cfg.AI.EnrichModel),
		zap.Int("worker_count", workerCount(cfg.AI)),
	)

	// ── 数据库 ──
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	defer entClient.Close()

	// ── Redis ──
	redisClient := commonRedis.Init(cfg.Redis, log)
	defer redisClient.Close()

	// ── 仓储层 ──
	snippetRepo := snippetstore.New(entClient)
	tagRepo := tagstore.New(entClient)
	aiMetadataRepo := aimetadatastore.New(entClient)
	callLogRepo := aicallogstore.New(entClient)

	// ── ID Generator（call log 记录需要分配 ID） ──
	idgenClient, err := idgen.New(cfg.IDGenerator.Addr)
	if err != nil {
		log.Fatal("连接 id-generator 失败", zap.String("addr", cfg.IDGenerator.Addr), zap.Error(err))
	}
	defer idgenClient.Close()

	// ── LLM Client ──
	llmClient := newLLMClient(cfg.AI)

	// ── 分布式锁（防止并发 worker 重复处理同一 snippet） ──
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

	// ── 启动 Worker ──
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := workerCount(cfg.AI)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			consumerName := fmt.Sprintf("worker-%d", workerID)

			log.Info("Worker 已启动", zap.Int("worker_id", workerID))

			err := subscriber.Subscribe(ctx, event.TopicSnippetSaved, "ai-enricher", consumerName,
				func(ctx context.Context, data []byte) error {
					var payload event.SnippetSavedPayload
					if err := json.Unmarshal(data, &payload); err != nil {
						log.Error("反序列化事件失败", zap.Error(err), zap.ByteString("data", data))
						return nil // 不重试格式错误的消息
					}

					log.Info("开始处理 snippet 增值",
						zap.Int64("snippet_id", payload.SnippetID),
						zap.String("action", payload.Action),
					)

					_, err := enrichSvc.Enrich(ctx, payload.SnippetID)
					if err != nil {
						log.Error("增值处理失败",
							zap.Int64("snippet_id", payload.SnippetID),
							zap.Error(err),
						)
						return err // 返回 error → 不 ACK → 留待重试
					}
					return nil
				},
			)
			if err != nil && err != context.Canceled {
				log.Error("Worker 异常退出", zap.Int("worker_id", workerID), zap.Error(err))
			}
		}(i)
	}

	// ── 优雅退出 ──
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("收到退出信号，正在停止 Worker...", zap.String("signal", sig.String()))

	cancel()
	wg.Wait()
	log.Info("AI Worker 已停止")
}

// newLLMClient 根据配置创建对应的 LLM Client。
func newLLMClient(cfg config.AIConfig) llm.Client {
	switch cfg.Provider {
	case "anthropic":
		return llm.NewAnthropicClient(cfg.BaseURL, cfg.APIKey)
	default: // "openai", "ollama", "qwen", 或任意 OpenAI Compatible 端点
		return llm.NewOpenAICompatClient(cfg.BaseURL, cfg.APIKey)
	}
}

// workerCount 返回 Worker 并发数（默认 4）。
func workerCount(cfg config.AIConfig) int {
	if cfg.WorkerCount > 0 {
		return cfg.WorkerCount
	}
	return 4
}

func validateAIConfig(cfg config.AIConfig) error {
	if cfg.EnrichModel == "" {
		return fmt.Errorf("AI_ENRICH_MODEL 不能为空")
	}
	return nil
}
