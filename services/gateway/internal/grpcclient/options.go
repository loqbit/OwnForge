package grpcclient

import (
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DefaultDialOptions returns the standard production-grade set of gRPC client DialOptions.
//
// Includes:
//   - insecure transport for internal traffic where TLS is not needed
//   - OTel tracing with automatic TraceID propagation
//   - keepalive pings to prevent idle connections from being closed by intermediaries
//   - circuit breaker for fast failure when downstream services keep failing
//   - automatic retries for recoverable errors only
//
// target identifies the downstream service, and each target has its own breaker state.
func DefaultDialOptions(target string) []grpc.DialOption {
	const maxMsgSize = 16 << 20 // 16 MB — must support up to 10 MB file uploads plus protobuf encoding overhead

	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMsgSize),
			grpc.MaxCallSendMsgSize(maxMsgSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // send a keepalive ping every 10 seconds to detect disconnects quickly
			Timeout:             3 * time.Second,  // consider the connection broken if no response arrives within 3 seconds
			PermitWithoutStream: true,             // keep sending heartbeats even without active RPCs
		}),
		grpc.WithDefaultServiceConfig(serviceConfig()),
		// circuit-breaker interceptor to decide whether to fast-fail before retrying
		grpc.WithChainUnaryInterceptor(CircuitBreakerInterceptor(target)),
	}
}

// serviceConfig returns the gRPC client's service-config JSON.
//
// Includes:
//   - round_robin load balancing with a K8s Headless Service so gRPC distributes requests across backend pods,
//     solving the classic issue where HTTP/2 long-lived connections defeat L4 load balancing.
//   - retry policy: retry up to 3 times (4 total attempts including the first call) with exponential backoff, only for UNAVAILABLE and DEADLINE_EXCEEDED.
func serviceConfig() string {
	return `{
		"loadBalancingConfig": [{"round_robin": {}}],
		"methodConfig": [{
			"name": [{"service": ""}],
			"timeout": "3s",
			"retryPolicy": {
				"maxAttempts": 4,
				"initialBackoff": "0.1s",
				"maxBackoff": "1s",
				"backoffMultiplier": 2.0,
				"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
			}
		}]
	}`
}
