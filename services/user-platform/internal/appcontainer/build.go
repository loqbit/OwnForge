package appcontainer

import (
	"github.com/redis/go-redis/v9"

	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/config"
	platformidgen "github.com/loqbit/ownforge/services/user-platform/internal/platform/idgen"
	"go.uber.org/zap"
)

// Build constructs the application runtime container from infrastructure clients.
func Build(cfg *config.Config, entClient *ent.Client, redisClient *redis.Client, idgenClient platformidgen.Client, log *zap.Logger) *Container {
	stores := buildStores(entClient, redisClient, idgenClient)
	support := buildSupport(cfg, redisClient, log)
	services := buildServices(cfg, stores, support, log)
	userHandler := buildUserHandler(services, log)

	return &Container{
		UserService:    services.userService,
		ProfileService: services.profileService,
		AuthService:    services.authService,
		UserHandler:    userHandler,
		JWTManager:     support.jwtManager,
	}
}
