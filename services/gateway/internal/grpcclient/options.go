package grpcclient

import (
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DefaultDialOptions 返回生产级 gRPC 客户端的标准 DialOption 集合。
//
// 包含：
//   - 明文传输（内网通信，不需要 TLS）
//   - OTel 链路追踪（自动传播 TraceID）
//   - KeepAlive 心跳（防止空闲连接被中间设备关闭）
//   - 熔断器（下游持续故障时快速失败）
//   - 自动重试（仅对可恢复的错误生效）
//
// target 用于标识下游服务，每个 target 有独立的熔断器状态。
func DefaultDialOptions(target string) []grpc.DialOption {
	const maxMsgSize = 16 << 20 // 16 MB — 需要支持最大 10MB 文件上传 + protobuf 编码开销

	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMsgSize),
			grpc.MaxCallSendMsgSize(maxMsgSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 每 10 秒发一次心跳，快速检测断连
			Timeout:             3 * time.Second,  // 3 秒没响应视为连接断开
			PermitWithoutStream: true,             // 即使没有活跃 RPC 也保持心跳
		}),
		grpc.WithDefaultServiceConfig(serviceConfig()),
		// 熔断器拦截器 — 在重试之前判断是否需要快速失败
		grpc.WithChainUnaryInterceptor(CircuitBreakerInterceptor(target)),
	}
}

// serviceConfig 返回 gRPC 客户端的 Service Config JSON 配置。
//
// 包含：
//   - round_robin 负载均衡：配合 K8s Headless Service，让 gRPC 将请求轮询分发到所有后端 Pod，
//     解决 HTTP/2 长连接导致 L4 负载均衡失效的经典问题。
//   - 重试策略：最多重试 3 次（含首次调用共 4 次），指数退避，仅对 UNAVAILABLE 和 DEADLINE_EXCEEDED 生效。
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
