package lsp

import (
	"context"
	stderrors "errors"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type phase int

const (
	phaseNew phase = iota
	phaseRunning
	phaseShutDown
)

// ErrExitWithoutShutdown is what Serve returns when the client sent exit before shutdown.
var ErrExitWithoutShutdown = stderrors.New("exit notification received before shutdown")

type lifecycle struct {
	mu     sync.Mutex
	phase  phase
	exited chan struct{}
	clean  bool
	once   sync.Once
}

func newLifecycle() *lifecycle { return &lifecycle{exited: make(chan struct{})} }

func (l *lifecycle) admit(r *jsonrpc2.Request) (drop bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	m := r.Method()
	if m == protocol.MethodExit {
		return false, nil
	}
	switch {
	case m == protocol.MethodInitialize && l.phase != phaseNew:
		err = jsonrpc2.NewError(jsonrpc2.InvalidRequest, "server is already initialized")
	case m == protocol.MethodInitialize:
		return false, nil
	case l.phase == phaseNew:
		err = jsonrpc2.NewError(jsonrpc2.ServerNotInitialized, "server is not initialized")
	case l.phase == phaseShutDown:
		err = jsonrpc2.NewError(jsonrpc2.InvalidRequest, "server is shut down")
	default:
		return false, nil
	}
	return !r.IsCall(), err
}

func (l *lifecycle) after(method string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case method == protocol.MethodInitialize && err == nil:
		l.phase = phaseRunning
	case method == protocol.MethodShutdown && err == nil && l.phase == phaseRunning:
		l.phase = phaseShutDown
	case method == protocol.MethodExit:
		l.clean = l.phase == phaseShutDown
		l.once.Do(func() { close(l.exited) })
	}
}

func (l *lifecycle) gate(next jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, r *jsonrpc2.Request) (any, error) {
		if drop, err := l.admit(r); err != nil {
			if drop {
				return nil, nil
			}
			return nil, err
		}
		resp, err := next(ctx, r)
		l.after(r.Method(), err)
		return resp, err
	}
}

func (l *lifecycle) exitErr() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clean {
		return nil
	}
	return ErrExitWithoutShutdown
}
