package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dop251/goja"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type reactWorker struct {
	vm     *goja.Runtime
	render goja.Callable
}

type reactRenderer struct {
	program *goja.Program
	workers chan *reactWorker
	mu      sync.Mutex
	flights map[[sha256.Size]byte]*renderFlight
}

type renderFlight struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	html    []byte
	err     error
}

const maxRenderFlights = 128

func newReactRenderer(filename string) (*reactRenderer, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read React renderer: %w", err)
	}
	program, err := goja.Compile("renderer.js", string(source), true)
	if err != nil {
		return nil, fmt.Errorf("compile React renderer: %w", err)
	}
	r := &reactRenderer{program: program, workers: make(chan *reactWorker, 2)}
	for range cap(r.workers) {
		worker, err := r.newWorker()
		if err != nil {
			return nil, err
		}
		r.workers <- worker
	}
	return r, nil
}

func (r *reactRenderer) newWorker() (*reactWorker, error) {
	vm := goja.New()
	vm.SetMaxCallStackSize(2048)
	if _, err := vm.RunProgram(r.program); err != nil {
		return nil, fmt.Errorf("initialize React renderer: %w", err)
	}
	exports := vm.Get("ManifestsRenderer")
	if exports == nil || goja.IsUndefined(exports) || goja.IsNull(exports) {
		return nil, errors.New("react renderer exports missing")
	}
	render, ok := goja.AssertFunction(exports.ToObject(vm).Get("renderPage"))
	if !ok {
		return nil, errors.New("react renderPage export missing")
	}
	return &reactWorker{vm: vm, render: render}, nil
}

func (r *reactRenderer) render(ctx context.Context, data []byte) (html []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ctx, span := otel.Tracer("manifests.io/render").Start(ctx, "react.render")
	defer func() {
		if err != nil {
			span.SetAttributes(attribute.String("render.failure", renderFailureKind(err)))
			span.SetStatus(codes.Error, "React rendering failed")
		}
		span.End()
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.renderShared(ctx, data)
}

func (r *reactRenderer) renderShared(ctx context.Context, data []byte) ([]byte, error) {
	key := sha256.Sum256(data)
	r.mu.Lock()
	flight := r.flights[key]
	if flight == nil {
		if len(r.flights) >= maxRenderFlights {
			r.mu.Unlock()
			return r.renderWorker(ctx, data)
		}
		// Shared work outlives one caller but stops when its last waiter leaves.
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		flight = &renderFlight{done: make(chan struct{}), cancel: cancel}
		if r.flights == nil {
			r.flights = make(map[[sha256.Size]byte]*renderFlight)
		}
		r.flights[key] = flight
		data = bytes.Clone(data)
		go func() {
			html, err := r.renderWorker(workCtx, data)
			cancel()
			r.mu.Lock()
			flight.html, flight.err = html, err
			if r.flights[key] == flight {
				delete(r.flights, key)
			}
			close(flight.done)
			r.mu.Unlock()
		}()
	}
	flight.waiters++
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			if r.flights[key] == flight {
				delete(r.flights, key)
			}
			flight.cancel()
		}
		r.mu.Unlock()
	}()
	select {
	case <-flight.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return bytes.Clone(flight.html), flight.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *reactRenderer) renderWorker(ctx context.Context, data []byte) (html []byte, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var worker *reactWorker
	select {
	case worker = <-r.workers:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() {
		if err != nil {
			worker = nil
		}
		r.workers <- worker
	}()
	if worker == nil {
		worker, err = r.newWorker()
		if err != nil {
			return nil, err
		}
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		worker.vm.Interrupt(ctx.Err())
		close(interrupted)
	})
	result, err := worker.render(goja.Undefined(), worker.vm.ToValue(string(data)))
	if !stop() {
		<-interrupted
	}
	worker.vm.ClearInterrupt()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte(result.String()), nil
}

// Only emit fixed categories: JavaScript errors can contain page or request data.
func renderFailureKind(err error) string {
	var exception *goja.Exception
	var overflow *goja.StackOverflowError
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.As(err, &overflow):
		return "stack_overflow"
	case errors.As(err, &exception):
		return "javascript_exception"
	default:
		return "internal"
	}
}
