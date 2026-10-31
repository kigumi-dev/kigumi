package lsp

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestOrderedHandlerCancelsAheadOfQueuedMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, b := net.Pipe()
	server, client := jsonrpc2.NewConn(jsonrpc2.NewStream(a)), jsonrpc2.NewConn(jsonrpc2.NewStream(b))
	t.Cleanup(func() { server.Close(); client.Close() })
	started := make(chan jsonrpc2.ID, 1)
	second := make(chan bool, 1)
	var finished atomic.Bool
	handler := func(ctx context.Context, request *jsonrpc2.Request) (any, error) {
		switch request.Method() {
		case "first":
			started <- request.ID()
			<-ctx.Done()
			finished.Store(true)
			return nil, ctx.Err()
		case "second":
			second <- finished.Load()
		}
		return nil, nil
	}
	server.Go(ctx, protocol.CancelHandler(orderedHandler(handler)))
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	firstResult := make(chan error, 1)
	go func() {
		_, err := client.Call(ctx, "first", nil, nil)
		firstResult <- err
	}()
	var id jsonrpc2.ID
	select {
	case id = <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := client.Notify(ctx, "second", nil); err != nil {
		t.Fatal(err)
	}
	number, _ := id.Number()
	if err := client.Notify(ctx, protocol.MethodCancelRequest, map[string]any{"id": number}); err != nil {
		t.Fatal(err)
	}
	select {
	case inOrder := <-second:
		if !inOrder {
			t.Fatal("second message overtook the first")
		}
	case <-ctx.Done():
		t.Fatal("cancellation waited behind queued work")
	}
	select {
	case err := <-firstResult:
		if err == nil {
			t.Fatal("the first request was not cancelled")
		}
	case <-ctx.Done():
		t.Fatal("cancelled request did not finish")
	}
}
