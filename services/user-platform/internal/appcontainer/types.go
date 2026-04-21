package appcontainer

import (
	"github.com/luckysxx/common/ratelimiter"
	"github.com/ownforge/ownforge/services/user-platform/internal/auth"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/smsauth"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	applicationrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/application"
	infrarepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/infra"
	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	authservice "github.com/ownforge/ownforge/services/user-platform/internal/service/auth"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/handler"
)

// Container 承载应用运行所需的核心服务与传输层适配器。
type Container struct {
	UserService    accountservice.UserService
	ProfileService accountservice.ProfileService
	AuthService    authservice.AuthService
	UserHandler    *handler.UserHandler
	JWTManager     *auth.JWTManager
}

type storeSet struct {
	userRepo       accountrepo.UserRepository
	identityRepo   accountrepo.UserIdentityRepository
	profileRepo    accountrepo.ProfileRepository
	outboxStore    infrarepo.EventOutboxWriter
	tm             infrarepo.TransactionManager
	sessionRepo    sessionrepo.SessionRepository
	authzRepo      applicationrepo.UserAppAuthorizationRepository
	ssoSessionRepo sessionrepo.SsoSessionRepository
	appSessionRepo sessionrepo.AppSessionRepository
	phoneCodeRepo  sessionrepo.PhoneCodeRepository
}

type supportSet struct {
	jwtManager    *auth.JWTManager
	limiter       ratelimiter.Limiter
	smsAuthSender smsauth.Sender
}

type serviceSet struct {
	userService    accountservice.UserService
	profileService accountservice.ProfileService
	authService    authservice.AuthService
}
