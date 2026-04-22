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

// circuitBreakerTransport wraps http.RoundTripper and checks the circuit-breaker state before forwarding a request
type circuitBreakerTransport struct {
	http.RoundTripper
	Breaker *gobreaker.CircuitBreaker
}

// this RoundTrip function is always called when proxying a network request
func (c *circuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// cb.Execute contains the core circuit-breaker state machine:
	// If the breaker is open, it does not run the enclosed logic at all and returns a special error immediately.
	res, err := c.Breaker.Execute(func() (interface{}, error) {
		resp, err := c.RoundTripper.RoundTrip(req)
		if err != nil {
			return nil, err // network failure counts as a failure
		}
		// If the downstream service repeatedly returns HTTP 500/502/503/504, treat that as a failure too.
		if resp.StatusCode >= 500 {
			return resp, errors.New("downstream service returned a severe 5xx error")
		}
		// A normal 200 or 400 response counts as success because business validation failures are not service outages.
		return resp, nil
	})
	// The circuit breaker is in protection mode and has tripped.
	if err == gobreaker.ErrOpenState {
		// Return a clean and fast HTTP 503 response.
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

// NewReverseProxy Wrap the standard library to create a reverse proxy with a high-performance connection pool.
func NewReverseProxy(targetHost string) *httputil.ReverseProxy {
	// parse the target downstream service address
	targetURL, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("failed to parse target address: %v", err)
	}

	// This is the native reverse-proxy engine.
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 1. Create the underlying high-performance connection-pool transport
	baseTransport := &http.Transport{
		MaxIdleConns:        1000,             // maximum idle connections for the entire pool
		MaxIdleConnsPerHost: 200,              // maximum number of idle keepalive connections per downstream host
		IdleConnTimeout:     90 * time.Second, // how long an idle keepalive connection can stay open and be reused
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   true,
	}

	// 2. initialize the Sony circuit breaker
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ProxyBreaker-" + targetHost,
		MaxRequests: 3,                // allow at most 3 probe requests while half-open to test the downstream
		Interval:    10 * time.Second, // the statistics window is 10 seconds
		Timeout:     5 * time.Second,  // after tripping, wait 5 seconds before switching to half-open for probing
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip when requests exceed 10 within a second and the failure rate (errors or 5xx) exceeds 50%.
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
