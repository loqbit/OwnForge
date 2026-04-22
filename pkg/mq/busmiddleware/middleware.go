package busmiddleware

import "github.com/ownforge/ownforge/pkg/mq/bus"

// Middleware 允许在 bus.Handler 外层叠加通用处理逻辑。
type Middleware func(next bus.Handler) bus.Handler

// Chain 按传入顺序组装中间件。
func Chain(handler bus.Handler, middlewares ...Middleware) bus.Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}
