package busmiddleware

import "github.com/loqbit/ownforge/pkg/mq/bus"

// Middleware lets generic processing logic wrap a bus.Handler.
type Middleware func(next bus.Handler) bus.Handler

// Chain composes middleware in the order provided.
func Chain(handler bus.Handler, middlewares ...Middleware) bus.Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}
