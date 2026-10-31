package lsp

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Connect serves messages in wire order while cancellation remains independent of queued work.
func (s *Server) Connect(ctx context.Context, stream jsonrpc2.Stream) jsonrpc2.Conn {
	conn := jsonrpc2.NewConn(stream, jsonrpc2.WithCodec(protocolCodec{}))
	ctx = protocol.WithClient(ctx, protocol.ClientDispatcher(conn))
	handler := protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)
	conn.Go(ctx, protocol.CancelHandler(orderedHandler(handler)))
	return conn
}

func orderedHandler(handler jsonrpc2.Handler) jsonrpc2.Handler {
	tail := make(chan struct{})
	close(tail)
	return func(ctx context.Context, request *jsonrpc2.Request) (any, error) {
		previous := tail
		done := make(chan struct{})
		tail = done
		// The read loop reaches this assignment in order; Async then preserves the request and releases the reader.
		jsonrpc2.Async(ctx)
		defer close(done)
		<-previous
		return handler(ctx, request)
	}
}
