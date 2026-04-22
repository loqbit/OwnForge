package proxy

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// circuitBreakerTransport 包装了 http.RoundTripper，在转发请求前检查熔断器状态
type circuitBreakerTransport struct {
	http.RoundTripper
	Breaker *gobreaker.CircuitBreaker
}

// 代理转发网络请求时，一定会调用这个 RoundTrip 函数
func (c *circuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// cb.Execute 内部是熔断器的状态机核心：
	// 如果是断开（跳闸）状态，它压根不会执行大括号里的逻辑，而是直接返回一个特殊的错误
	res, err := c.Breaker.Execute(func() (interface{}, error) {
		resp, err := c.RoundTripper.RoundTrip(req)
		if err != nil {
			return nil, err // 网络不通，算作失败
		}
		// 如果下游服务疯狂抱歉返回 HTTP 500/502/503/504 服务器内部异常，我们也算作失败
		if resp.StatusCode >= 500 {
			return resp, errors.New("下游服务发生 5xx 严重异常")
		}
		// 正常返回 200 或者 400（业务层面的校验报错不能算宕机算作成功）
		return resp, nil
	})
	// 熔断器处于保护状态，直接拉闸了！
	if err == gobreaker.ErrOpenState {
		// 返回一个极其干净快速的 HTTP 503 结构
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable (Circuit Breaker OPEN)",
			Body:       http.NoBody,
			Request:    req,
			Header:     make(http.Header),
		}, nil
	}
	if err != nil && res == nil {
		return nil, err
	}
	return res.(*http.Response), nil
}

// NewReverseProxy 封装官方库，创建一个带极致性能连接池的反向代理引擎
func NewReverseProxy(targetHost string) *httputil.ReverseProxy {
	// 解析目标下游服务的地址
	targetURL, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("解析目标地址失败: %v", err)
	}

	// 这就是原生反向代理引擎
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 1. 创建底层的高性能连接池 Transport
	baseTransport := &http.Transport{
		MaxIdleConns:        1000,             // 整个连接池最大空闲连接数
		MaxIdleConnsPerHost: 200,              // 每个下游服务 Host 的最大空闲长连接数
		IdleConnTimeout:     90 * time.Second, // 长连接空闲多久不断开可以复用
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   true,
	}

	// 2. 初始化索尼阻断器
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ProxyBreaker-" + targetHost,
		MaxRequests: 3,                // 在半开状态下，最多放行 3 个探路请求去试探下游
		Interval:    10 * time.Second, // 统计时间窗口为 10 秒
		Timeout:     5 * time.Second,  // 一旦跳闸，等 5 秒后再切换为“半开”状态试探
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 跳闸规则：如果一秒内请求超过 10 次，并且失败率（抛出 err 或 5xx）超过 50%，就咔嚓跳闸断电！
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
	})

	proxy.Transport = &circuitBreakerTransport{
		RoundTripper: baseTransport,
		Breaker:      cb,
	}

	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	}

	return proxy
}
