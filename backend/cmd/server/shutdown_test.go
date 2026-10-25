package main

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeServer blocks in listen until shutdown is called, like http.Server.
type fakeServer struct {
	closed chan struct{}
	once   sync.Once
}

func newFakeServer() *fakeServer { return &fakeServer{closed: make(chan struct{})} }

func (f *fakeServer) listen() error {
	<-f.closed
	return http.ErrServerClosed
}

func (f *fakeServer) close() { f.once.Do(func() { close(f.closed) }) }

type recorder struct {
	mu    sync.Mutex
	steps []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, s)
}

func (r *recorder) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.steps...)
}

func TestRunServers_ShutdownOrder(t *testing.T) {
	rec := &recorder{}
	redirect, api := newFakeServer(), newFakeServer()
	servers := []namedServer{
		{name: "redirect", listen: redirect.listen, shutdown: func(context.Context) error {
			rec.add("shutdown:redirect")
			redirect.close()
			return nil
		}},
		{name: "api", listen: api.listen, shutdown: func(context.Context) error {
			rec.add("shutdown:api")
			api.close()
			return nil
		}},
	}
	stops := []namedStop{
		{name: "click-flusher", stop: func() { rec.add("stop:click-flusher") }},
		{name: "webhook-dispatcher", stop: func() { rec.add("stop:webhook-dispatcher") }},
		{name: "bulk-worker", stop: func() { rec.add("stop:bulk-worker") }},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runServers(ctx, servers, stops, time.Second) }()

	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, rec.get(), "nothing runs before the signal")
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("runServers did not return")
	}

	steps := rec.get()
	require.Len(t, steps, 5)
	assert.ElementsMatch(t, []string{"shutdown:redirect", "shutdown:api"}, steps[:2])
	assert.Equal(t, []string{"stop:click-flusher", "stop:webhook-dispatcher", "stop:bulk-worker"}, steps[2:])
}

func TestRunServers_ListenErrorReturns(t *testing.T) {
	rec := &recorder{}
	api := newFakeServer()
	bindErr := errors.New("listen tcp :8080: bind: address already in use")
	servers := []namedServer{
		{name: "redirect", listen: func() error { return bindErr }, shutdown: func(context.Context) error {
			rec.add("shutdown:redirect")
			return nil
		}},
		{name: "api", listen: api.listen, shutdown: func(context.Context) error {
			rec.add("shutdown:api")
			api.close()
			return nil
		}},
	}
	stops := []namedStop{{name: "w", stop: func() { rec.add("stop:w") }}}

	done := make(chan error, 1)
	go func() { done <- runServers(context.Background(), servers, stops, time.Second) }()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, bindErr)
	case <-time.After(2 * time.Second):
		t.Fatal("runServers did not return after a listen error")
	}
	// The remaining server and the workers are still shut down.
	steps := rec.get()
	assert.Contains(t, steps, "shutdown:api")
	assert.Equal(t, "stop:w", steps[len(steps)-1])
}

func TestRunServers_ShutdownTimeoutDoesNotHang(t *testing.T) {
	rec := &recorder{}
	stuck := newFakeServer()
	servers := []namedServer{
		// Simulates a server with a connection that never goes idle: Shutdown
		// returns only when its ctx expires (as http.Server.Shutdown does).
		{name: "stuck", listen: stuck.listen, shutdown: func(ctx context.Context) error {
			<-ctx.Done()
			stuck.close()
			return ctx.Err()
		}},
	}
	stops := []namedStop{{name: "w", stop: func() { rec.add("stop:w") }}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- runServers(ctx, servers, stops, 50*time.Millisecond) }()

	select {
	case err := <-done:
		require.NoError(t, err, "a shutdown timeout is logged, not returned")
	case <-time.After(2 * time.Second):
		t.Fatal("runServers hung on a shutdown that ignores the timeout")
	}
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, []string{"stop:w"}, rec.get(), "workers still stop after the timeout")
}
