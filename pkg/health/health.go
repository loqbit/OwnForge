// Package health provides standardized health-check endpoints.
//
// The design follows Kubernetes probe conventions:
//   - /healthz (Liveness)  — returns 200 as long as the process is alive, used to detect deadlocks or unrecoverable states
//   - /readyz  (Readiness) — returns 200 only when all dependencies are ready, used to determine whether the service can receive traffic
//
// Usage:
//
//	checker := health.NewChecker()
//	checker.AddCheck("postgres", func(ctx context.Context) error { return db.PingContext(ctx) })
//	checker.AddCheck("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
//	checker.Register(r) // r is *gin.Engine
package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CheckFunc is the health-check function for a single dependency; returning nil means healthy.
type CheckFunc func(ctx context.Context) error

// Checker manages a set of dependency checks and provides /healthz and /readyz endpoints.
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

// AddCheck registers a named dependency check.
// name identifies which dependency is unhealthy in the response.
func (c *Checker) AddCheck(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

// Register adds /healthz and /readyz routes to the Gin engine.
// These endpoints are registered before metrics and business middleware to avoid rate limiting or auth interception.
func (c *Checker) Register(r *gin.Engine) {
	r.GET("/healthz", c.liveness)
	r.GET("/readyz", c.readiness)
}

// RegisterHTTP adds /healthz and /readyz to the standard-library http.ServeMux.
// This is suitable for pure gRPC processes that expose a lightweight sidecar HTTP admin port.
func (c *Checker) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", c.livenessHTTP)
	mux.HandleFunc("/readyz", c.readinessHTTP)
}

// liveness is the liveness probe: it returns 200 as long as the process is running.
// K8s uses this to determine whether the container should be restarted.
func (c *Checker) liveness(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// readiness is the readiness probe: it checks all dependencies one by one and returns 200 only when all pass.
// K8s uses this to decide whether traffic should be routed to the Pod.
func (c *Checker) readiness(ctx *gin.Context) {
	status, body := c.readinessPayload(ctx.Request.Context())
	ctx.JSON(status, body)
}

// livenessHTTP is the standard-library HTTP version of the liveness probe.
func (c *Checker) livenessHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// readinessHTTP is the standard-library HTTP version of the readiness probe.
func (c *Checker) readinessHTTP(w http.ResponseWriter, r *http.Request) {
	status, body := c.readinessPayload(r.Context())
	writeJSON(w, status, body)
}

// Evaluate runs all checks and returns the overall health result plus per-check results.
// This method is suitable for reuse in non-HTTP scenarios such as native gRPC health checks and background inspections.
func (c *Checker) Evaluate(parent context.Context) (bool, map[string]string) {
	checks := c.snapshotChecks()

	checkCtx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	results := make(map[string]string, len(checks))
	allHealthy := true

	for name, fn := range checks {
		if err := fn(checkCtx); err != nil {
			results[name] = err.Error()
			allHealthy = false
		} else {
			results[name] = "ok"
		}
	}

	return allHealthy, results
}

// readinessPayload runs all checks and builds the readiness response payload.
func (c *Checker) readinessPayload(parent context.Context) (int, map[string]any) {
	allHealthy, results := c.Evaluate(parent)

	status := http.StatusOK
	overall := "ready"
	if !allHealthy {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}

	return status, map[string]any{
		"status": overall,
		"checks": results,
	}
}

func (c *Checker) snapshotChecks() map[string]CheckFunc {
	c.mu.RLock()
	defer c.mu.RUnlock()

	checks := make(map[string]CheckFunc, len(c.checks))
	for k, v := range c.checks {
		checks[k] = v
	}
	return checks
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil && !errors.Is(err, context.Canceled) {
		return
	}
}
