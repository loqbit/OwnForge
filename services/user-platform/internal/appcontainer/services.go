package appcontainer

import (
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	authservice "github.com/ownforge/ownforge/services/user-platform/internal/service/auth"
	"go.uber.org/zap"
)

func buildServices(cfg *config.Config, stores storeSet, support supportSet, log *zap.Logger) serviceSet {
	return serviceSet{
		userService: accountservice.NewUserService(accountservice.UserDependencies{
			TM:                  stores.tm,
			UserRepo:            stores.userRepo,
			IdentityRepo:        stores.identityRepo,
			ProfileRepo:         stores.profileRepo,
			SSOSessionRepo:      stores.ssoSessionRepo,
			AppSessionRepo:      stores.appSessionRepo,
			Outbox:              stores.outboxStore,
			Logger:              log,
			TopicUserRegistered: cfg.Kafka.TopicUserRegistered,
		}),
		profileService: accountservice.NewProfileService(accountservice.ProfileDependencies{
			ProfileRepo: stores.profileRepo,
			Logger:      log,
		}),
		authService: authservice.NewAuthService(authservice.AuthDependencies{
			TM:                  stores.tm,
			UserRepo:            stores.userRepo,
			IdentityRepo:        stores.identityRepo,
			ProfileRepo:         stores.profileRepo,
			AuthorizationRepo:   stores.authzRepo,
			SSOSessionRepo:      stores.ssoSessionRepo,
			AppSessionRepo:      stores.appSessionRepo,
			SessionCacheRepo:    stores.sessionRepo,
			PhoneCodeRepo:       stores.phoneCodeRepo,
			SMSAuthSender:       support.smsAuthSender,
			Outbox:              stores.outboxStore,
			JWTManager:          support.jwtManager,
			Limiter:             support.limiter,
			Logger:              log,
			AppEnv:              cfg.AppEnv,
			TopicUserRegistered: cfg.Kafka.TopicUserRegistered,
		}),
	}
}
