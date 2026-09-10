package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func coalescingRenderer(t *testing.T, script string, enter func(string)) *reactRenderer {
	t.Helper()
	file := filepath.Join(t.TempDir(), "renderer.js")
	if err := os.WriteFile(file, []byte(`var ManifestsRenderer = {renderPage: function(data) { if (typeof enter === "function") enter(data); `+script+` }};`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := newReactRenderer(file)
	if err != nil {
		t.Fatal(err)
	}
	for range cap(r.workers) {
		worker := <-r.workers
		if err := worker.vm.Set("enter", enter); err != nil {
			t.Fatal(err)
		}
		r.workers <- worker
	}
	return r
}

func awaitRenderState(t *testing.T, r *reactRenderer, ready func() bool) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		r.mu.Lock()
		ok := ready()
		r.mu.Unlock()
		if ok {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("render state did not settle")
		case <-tick.C:
		}
	}
}

func TestRenderCoalescesOnlyIdenticalConcurrentData(t *testing.T) {
	var executions atomic.Int32
	release := make(chan struct{})
	unfreeze := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unfreeze)
	r := coalescingRenderer(t, `return data;`, func(string) {
		executions.Add(1)
		<-release
	})
	results := make(chan error, 13)
	start := func(data string) {
		go func() {
			html, err := r.render(t.Context(), []byte(data))
			if err == nil && string(html) != data {
				err = errors.New("rendered a different caller's data")
			}
			results <- err
		}()
	}
	for range 12 {
		start("same")
	}
	key := sha256.Sum256([]byte("same"))
	awaitRenderState(t, r, func() bool { return r.flights[key] != nil && r.flights[key].waiters == 12 && executions.Load() == 1 })
	start("different")
	awaitRenderState(t, r, func() bool { return len(r.flights) == 2 && executions.Load() == 2 })
	unfreeze()
	for range 13 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.render(t.Context(), []byte("same")); err != nil {
		t.Fatal(err)
	}
	if executions.Load() != 3 {
		t.Fatal("completed render was retained as a persistent cache")
	}
}

func TestRenderCancellationPreservesOtherWaiters(t *testing.T) {
	release := make(chan struct{})
	unfreeze := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unfreeze)
	r := coalescingRenderer(t, `return data;`, func(string) { <-release })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	canceled, remaining := make(chan error, 1), make(chan error, 1)
	go func() { _, err := r.render(ctx, []byte("same")); canceled <- err }()
	key := sha256.Sum256([]byte("same"))
	awaitRenderState(t, r, func() bool { return r.flights[key] != nil && r.flights[key].waiters == 1 })
	go func() { _, err := r.render(t.Context(), []byte("same")); remaining <- err }()
	awaitRenderState(t, r, func() bool { return r.flights[key] != nil && r.flights[key].waiters == 2 })
	cancel()
	if err := <-canceled; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller returned %v", err)
	}
	awaitRenderState(t, r, func() bool { return r.flights[key] != nil && r.flights[key].waiters == 1 })
	unfreeze()
	if err := <-remaining; err != nil {
		t.Fatalf("one cancellation poisoned another caller: %v", err)
	}
}

func TestRenderAbandonedWorkIsCanceledAndFailuresAreNotRetained(t *testing.T) {
	started := make(chan struct{}, 1)
	r := coalescingRenderer(t, `if (data === "loop") while (true) {} if (data === "fail") throw new Error("failure"); return data;`, func(data string) {
		if data == "loop" {
			started <- struct{}{}
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { _, err := r.render(ctx, []byte("loop")); result <- err }()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller returned %v", err)
	}
	awaitRenderState(t, r, func() bool { return len(r.flights) == 0 && len(r.workers) == cap(r.workers) })
	for range 2 {
		if _, err := r.render(t.Context(), []byte("fail")); err == nil {
			t.Fatal("render failure was lost")
		}
		awaitRenderState(t, r, func() bool { return len(r.flights) == 0 })
	}
	html, err := r.render(t.Context(), []byte("recovered"))
	if err != nil || string(html) != "recovered" {
		t.Fatalf("workers failed to recover: %s, %v", html, err)
	}
}

type waitingContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *waitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestRenderFlightTrackingIsBounded(t *testing.T) {
	release := make(chan struct{})
	unfreeze := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unfreeze)
	r := coalescingRenderer(t, `return data;`, func(string) { <-release })
	results := make(chan error, maxRenderFlights+1)
	for i := range maxRenderFlights {
		go func() {
			_, err := r.render(t.Context(), []byte(strconv.Itoa(i)))
			results <- err
		}()
	}
	awaitRenderState(t, r, func() bool { return len(r.flights) == maxRenderFlights })
	ctx := &waitingContext{Context: t.Context(), waiting: make(chan struct{})}
	go func() {
		html, err := r.renderShared(ctx, []byte("overflow"))
		if err == nil && string(html) != "overflow" {
			err = errors.New("overflow request lost its data")
		}
		results <- err
	}()
	<-ctx.waiting
	r.mu.Lock()
	tracked, overflow := len(r.flights), r.flights[sha256.Sum256([]byte("overflow"))]
	r.mu.Unlock()
	if tracked != maxRenderFlights || overflow != nil {
		t.Fatal("distinct requests exceeded the in-flight tracking bound")
	}
	unfreeze()
	for range maxRenderFlights + 1 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	awaitRenderState(t, r, func() bool { return len(r.flights) == 0 && len(r.workers) == cap(r.workers) })
}
