package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"
)

// loggingInterceptor logs every RPC call with duration and status.
func loggingInterceptor() connect.Interceptor {
	return &loggingInt{}
}

type loggingInt struct{}

func (l *loggingInt) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		procedure := req.Spec().Procedure
		resp, err := next(ctx, req)
		duration := time.Since(start)

		status := "ok"
		if err != nil {
			status = connect.CodeOf(err).String()
		}
		fmt.Fprintf(os.Stderr, "%s %s %v\n", procedure, status, duration)

		return resp, err
	}
}

func (l *loggingInt) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (l *loggingInt) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
