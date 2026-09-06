package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dop251/goja"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type reactWorker struct {
	vm     *goja.Runtime
	render goja.Callable
}

type reactRenderer struct {
	program *goja.Program
	workers chan *reactWorker
}

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
		return nil, errors.New("React renderer exports missing")
	}
	render, ok := goja.AssertFunction(exports.ToObject(vm).Get("renderPage"))
	if !ok {
		return nil, errors.New("React renderPage export missing")
	}
	return &reactWorker{vm: vm, render: render}, nil
}

func (r *reactRenderer) render(ctx context.Context, data []byte) (html []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ctx, span := otel.Tracer("manifests.io/render").Start(ctx, "react.render")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, "React rendering failed")
		}
		span.End()
	}()
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
