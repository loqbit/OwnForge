package grpcclient

import (
	"context"
	"errors"
	"sync"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// circuitBreakers maintains a separate circuit breaker instance per target service address.
// Different downstream services have independent breaker states and do not affect each other.
var (
	cbMu     sync.Mutex
	breakers = make(map[string]*gobreaker.CircuitBreaker[any])
)

// getBreaker gets or creates the circuit breaker for the given target address.
func getBreaker(target string) *gobreaker.CircuitBreaker[any] {
	cbMu.Lock()
	defer cbMu.Unlock()

	if cb, ok := breakers[target]; ok {
		return cb
	}

	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name: "grpc-" + target,

		// Allow 3 probe requests in the half-open state
		MaxRequests: 3,

		// Statistics window: reset error counters every 10 seconds
		Interval: 10_000_000_000, // 10s in nanoseconds

		// After tripping, wait 5 seconds before entering the half-open state
		Timeout: 5_000_000_000, // 5s in nanoseconds

		// Trip when the failure rate is >= 50% over 10 requests
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},

		// Decide which errors count as failures: only network or server-side errors trip the breaker
		// Business errors such as parameter validation failures do not count as failures
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			st, ok := status.FromError(err)
			if !ok {
				return false // Non-gRPC errors are treated as failures
			}
			switch st.Code() {
			case codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.ResourceExhausted:
				return false // Server-side errors count as failures and are included in breaker statistics
			default:
				return true // Business errors such as InvalidArgument and NotFound do not count as failures
			}
		},
	})

	breakers[target] = cb
	return cb
}

// CircuitBreakerInterceptor returns a gRPC unary client interceptor that wraps each RPC call with circuit-breaker protection.
//
// Flow:
//
//	Closed (healthy) -> failure rate crosses threshold -> Open (tripped, fast-fail with 503)
//	                                   wait 5 seconds
//	                               Half-Open (allow 3 probes)
//	                                   probe succeeds
//	                               Closed (recovered)
func CircuitBreakerInterceptor(target string) grpc.UnaryClientInterceptor {
	cb := getBreaker(target)

	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		_, err := cb.Execute(func() (any, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			return nil, err
		})

		// Convert the error returned while the circuit breaker is open into gRPC Unavailable
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return status.Errorf(codes.Unavailable, "circuit breaker is open for %s", target)
		}

		return err
	}
}
