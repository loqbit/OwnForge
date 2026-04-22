package handler

import (
	"net/http"
	"strings"

	"github.com/ownforge/ownforge/services/gateway/internal/config"

	"github.com/gin-gonic/gin"
)

type ssoCookieManager struct {
	cfg config.SSOCookieConfig
}

func newSSOCookieManager(cfg config.SSOCookieConfig) *ssoCookieManager {
	return &ssoCookieManager{cfg: cfg}
}

func (m *ssoCookieManager) set(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     m.cfg.Name,
		Value:    token,
		Path:     defaultCookiePath(m.cfg.Path),
		Domain:   m.cfg.Domain,
		MaxAge:   m.cfg.MaxAge,
		HttpOnly: m.cfg.HTTPOnly,
		Secure:   m.cfg.Secure,
		SameSite: parseSameSite(m.cfg.SameSite),
	})
}

func (m *ssoCookieManager) clear(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     m.cfg.Name,
		Value:    "",
		Path:     defaultCookiePath(m.cfg.Path),
		Domain:   m.cfg.Domain,
		MaxAge:   -1,
		HttpOnly: m.cfg.HTTPOnly,
		Secure:   m.cfg.Secure,
		SameSite: parseSameSite(m.cfg.SameSite),
	})
}

func (m *ssoCookieManager) get(c *gin.Context) (string, bool) {
	raw, err := c.Cookie(m.cfg.Name)
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", false
	}
	return token, true
}

func defaultCookiePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "/"
	}
	return path
}

func parseSameSite(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		fallthrough
	default:
		return http.SameSiteLaxMode
	}
}
