package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/observability"
	"github.com/TheOutdoorProgrammer/manifests.io/internal/upstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	root := flag.String("root", ".", "repository containing the schema corpus")
	apply := flag.Bool("apply", false, "download, validate and add new versions; default only discovers updates")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err := run(ctx, *root, *apply); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, root string, apply bool) error {
	shutdown, err := observability.SetupWithWriter(ctx, "schema-updater", os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		flush, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := shutdown(flush); err != nil {
			slog.Error("schema updater telemetry shutdown failed")
		}
	}()
	ctx, span := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.update")
	defer span.End()
	sources, err := upstream.Sources()
	if err != nil {
		return err
	}
	client := upstream.NewClient(os.Getenv("GITHUB_TOKEN"))
	updates, err := client.Discover(ctx, root, sources)
	if err != nil {
		span.SetStatus(codes.Error, "discovery failed")
		slog.ErrorContext(ctx, "schema release discovery failed")
		return err
	}
	if apply {
		if err := client.Apply(ctx, root, updates); err != nil {
			span.SetStatus(codes.Error, "update failed")
			slog.ErrorContext(ctx, "schema update failed")
			return err
		}
	}
	slog.InfoContext(ctx, "schema update completed", "schema.updates", len(updates))
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(updates)
}
