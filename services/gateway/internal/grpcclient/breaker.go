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

// circuitBreakers 按目标服务地址维护各自的熔断器实例。
// 不同的下游服务有独立的熔断状态，互不影响。
var (
	cbMu       sync.Mutex
	breakers   = make(map[string]*gobreaker.CircuitBreaker[any])
)

// getBreaker 获取或创建指定目标地址的熔断器。
func getBreaker(target string) *gobreaker.CircuitBreaker[any] {
	cbMu.Lock()
	defer cbMu.Unlock()

	if cb, ok := breakers[target]; ok {
		return cb
	}

	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name: "grpc-" + target,

		// Half-Open 状态下允许 3 个探测请求通过
		MaxRequests: 3,

		// 统计窗口：每 10 秒清零一次错误计数
		Interval: 10_000_000_000, // 10s in nanoseconds

		// 熔断后等待 5 秒进入 Half-Open 状态
		Timeout: 5_000_000_000, // 5s in nanoseconds

		// 跳闸条件：10 次请求中失败率 ≥ 50%
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},

		// 判断哪些错误算"失败"：仅网络/服务端错误触发熔断
		// 业务错误（如参数校验失败）不算失败
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			st, ok := status.FromError(err)
			if !ok {
				return false // 非 gRPC 错误视为失败
			}
			switch st.Code() {
			case codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.ResourceExhausted:
				return false // 服务端错误 → 算失败，计入熔断统计
			default:
				return true // 业务错误（InvalidArgument, NotFound 等）→ 不算失败
			}
		},
	})

	breakers[target] = cb
	return cb
}

// CircuitBreakerInterceptor 返回一个 gRPC 客户端一元拦截器，为每次 RPC 调用包裹熔断保护。
//
// 工作流程：
//
//	Closed（正常）→ 失败率达阈值 → Open（熔断，快速失败 503）
//	                                   ↓ 等 5 秒
//	                               Half-Open（放 3 个探测）
//	                                   ↓ 探测成功
//	                               Closed（恢复正常）
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

		// 熔断器处于 Open 状态时返回的错误，转换为 gRPC Unavailable
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return status.Errorf(codes.Unavailable, "circuit breaker is open for %s", target)
		}

		return err
	}
}
