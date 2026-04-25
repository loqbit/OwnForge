package appcontainer

import (
	"github.com/redis/go-redis/v9"

	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"github.com/loqbit/ownforge/services/identity/internal/auth"
	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/platform/config"
	platformidgen "github.com/loqbit/ownforge/services/identity/internal/platform/idgen"
	"github.com/loqbit/ownforge/services/identity/internal/platform/smsauth"
	entaccountstore "github.com/loqbit/ownforge/services/identity/internal/store/entstore/account"
	entapplicationstore "github.com/loqbit/ownforge/services/identity/internal/store/entstore/application"
	entinfrastore "github.com/loqbit/ownforge/services/identity/internal/store/entstore/infra"
	entsessionstore "github.com/loqbit/ownforge/services/identity/internal/store/entstore/session"
	redisphonecodestore "github.com/loqbit/ownforge/services/identity/internal/store/redisstore/phonecode"
	redissessionstore "github.com/loqbit/ownforge/services/identity/internal/store/redisstore/session"
	"go.uber.org/zap"
)

func buildStores(entClient *ent.Client, redisClient *redis.Client, idgenClient platformidgen.Client) storeSet {
	return storeSet{
		userRepo:       entaccountstore.NewUserStore(entClient, idgenClient),
		identityRepo:   entaccountstore.NewUserIdentityStore(entClient),
		profileRepo:    entaccountstore.NewProfileStore(entClient, idgenClient),
		outboxStore:    entinfrastore.NewEventOutboxStore(entClient),
		tm:             entinfrastore.NewTransactionManager(entClient),
		sessionRepo:    redissessionstore.NewSessionStore(redisClient),
		authzRepo:      entapplicationstore.NewUserAppAuthorizationStore(entClient),
		ssoSessionRepo: entsessionstore.NewSsoSessionStore(entClient),
		appSessionRepo: entsessionstore.NewAppSessionStore(entClient),
		phoneCodeRepo:  redisphonecodestore.NewPhoneCodeStore(redisClient),
	}
}

func buildSupport(cfg *config.Config, redisClient *redis.Client, log *zap.Logger) supportSet {
	return supportSet{
		jwtManager:    auth.NewJWTManager(cfg.JWT.Secret),
		limiter:       ratelimiter.NewFixedWindowLimiter(redisClient, log),
		smsAuthSender: smsauth.NewAliyunSender(cfg.SMSAuth, log),
	}
}
