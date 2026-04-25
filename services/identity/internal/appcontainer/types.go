package appcontainer

import (
	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"github.com/loqbit/ownforge/services/identity/internal/auth"
	"github.com/loqbit/ownforge/services/identity/internal/platform/smsauth"
	accountrepo "github.com/loqbit/ownforge/services/identity/internal/repository/account"
	applicationrepo "github.com/loqbit/ownforge/services/identity/internal/repository/application"
	infrarepo "github.com/loqbit/ownforge/services/identity/internal/repository/infra"
	sessionrepo "github.com/loqbit/ownforge/services/identity/internal/repository/session"
	accountservice "github.com/loqbit/ownforge/services/identity/internal/service/account"
	authservice "github.com/loqbit/ownforge/services/identity/internal/service/auth"
	"github.com/loqbit/ownforge/services/identity/internal/transport/http/server/handler"
)

// Container holds the core services and transport adapters required to run the application.
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
