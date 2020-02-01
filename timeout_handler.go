package lambdamux

import (
	"context"
	"time"
)

type timeoutHandler struct {
	Timeout time.Duration
	Handler ResourceHandler
}

// ResourceHandlerWithTimeout provides a resource handler with a configured
// timeout that will be invoked per serve resource.
func ResourceHandlerWithTimeout(dur time.Duration, handler ResourceHandler) ResourceHandler {
	return timeoutHandler{
		Timeout: dur,
		Handler: handler,
	}
}

// ServeResources wraps a resource handler with a timeout
func (h timeoutHandler) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	var cancelFn func()

	ctx, cancelFn = context.WithTimeout(ctx, h.Timeout)
	defer cancelFn()

	done := make(chan struct{})
	go func() {
		resp, err = h.Handler.ServeResource(ctx, req)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return APIGatewayProxyResponse{}, ctx.Err()
	case <-done:
		return resp, err
	}
}
