// Package health 提供标准化的健康检查端点。
//
// 设计遵循 Kubernetes 探针规范：
//   - /healthz (Liveness)  — 进程存活即返回 200，用于检测死锁或不可恢复的状态
//   - /readyz  (Readiness) — 所有依赖就绪才返回 200，用于判断是否可以接收流量
//
// 用法：
//
//	checker := health.NewChecker()
//	checker.AddCheck("postgres", func(ctx context.Context) error { return db.PingContext(ctx) })
//	checker.AddCheck("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
//	checker.Register(r) // r 是 *gin.Engine
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

// CheckFunc 是单个依赖的健康检查函数，返回 nil 表示健康。
type CheckFunc func(ctx context.Context) error

// Checker 管理一组依赖检查项，提供 /healthz 和 /readyz 端点。
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// NewChecker 创建一个新的健康检查器。
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

// AddCheck 注册一个命名的依赖检查项。
// name 用于在响应中标识具体哪个依赖不健康。
func (c *Checker) AddCheck(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

// Register 将 /healthz 和 /readyz 路由注册到 Gin 引擎上。
// 这两个端点注册在 metrics 和业务中间件之前，避免被限流或鉴权拦截。
func (c *Checker) Register(r *gin.Engine) {
	r.GET("/healthz", c.liveness)
	r.GET("/readyz", c.readiness)
}

// RegisterHTTP 将 /healthz 和 /readyz 注册到标准库 http.ServeMux。
// 适用于纯 gRPC 进程旁路启动一个轻量 HTTP 管理端口的场景。
func (c *Checker) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", c.livenessHTTP)
	mux.HandleFunc("/readyz", c.readinessHTTP)
}

// liveness 存活探针：进程在跑就返回 200。
// K8s 用这个判断是否需要重启容器。
func (c *Checker) liveness(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// readiness 就绪探针：逐一检查所有依赖，全部通过才返回 200。
// K8s 用这个判断是否把流量路由到该 Pod。
func (c *Checker) readiness(ctx *gin.Context) {
	status, body := c.readinessPayload(ctx.Request.Context())
	ctx.JSON(status, body)
}

// livenessHTTP 是标准库 http 版本的存活探针。
func (c *Checker) livenessHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

// readinessHTTP 是标准库 http 版本的就绪探针。
func (c *Checker) readinessHTTP(w http.ResponseWriter, r *http.Request) {
	status, body := c.readinessPayload(r.Context())
	writeJSON(w, status, body)
}

// Evaluate 执行所有检查项，返回整体是否健康以及逐项结果。
// 这个方法适合给 gRPC 原生健康检查、后台巡检等非 HTTP 场景复用。
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

// readinessPayload 统一执行所有检查项，生成 readiness 响应体。
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
