package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/observability"
	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
	"github.com/TheOutdoorProgrammer/manifests.io/internal/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := flag.String("data", env("DATA_DIR", "."), "schema corpus directory")
	webDir := flag.String("web", env("WEB_DIR", "frontend/dist"), "built frontend directory")
	renderDir := flag.String("rendered", env("RENDER_DIR", "frontend/prerender"), "prerendered documentation directory")
	publicDir := flag.String("public", env("PUBLIC_DIR", "public"), "public assets directory")
	export := flag.Bool("export", false, "write pages as newline-delimited JSON and exit")
	static := flag.Bool("export-static", false, "write complete static release records as newline-delimited JSON and exit")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !*export && !*static {
		shutdown, err := observability.Setup(ctx, version)
		if err != nil {
			return err
		}
		defer func() {
			flush, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := shutdown(flush); err != nil {
				slog.Error("telemetry shutdown failed")
			}
		}()
	}
	loadCtx, span := otel.Tracer("manifests.io/schema").Start(ctx, "catalog.load")
	catalog, err := schema.Load(*dataDir)
	if err != nil {
		span.SetStatus(codes.Error, "")
		span.End()
		return fmt.Errorf("load schemas: %w", err)
	}
	if !*export && !*static {
		slog.InfoContext(loadCtx, "schema catalog loaded", "documents", len(catalog.Routes()))
	}
	span.End()
	if *static {
		return server.ExportStatic(ctx, catalog, os.Getenv("SITE_URL"), os.Stdout)
	}
	if *export {
		encoder := json.NewEncoder(os.Stdout)
		for _, query := range catalog.Routes() {
			page, err := catalog.Page(query)
			if err != nil {
				return fmt.Errorf("export page: %w", err)
			}
			if err := encoder.Encode(struct {
				Page schema.Page `json:"page"`
				File string      `json:"file"`
			}{page, server.RenderFilename(page.Canonical)}); err != nil {
				return err
			}
		}
		return nil
	}
	handler, err := server.New(catalog, server.Config{WebDir: *webDir, RenderDir: *renderDir, PublicDir: *publicDir, SiteURL: os.Getenv("SITE_URL")})
	if err != nil {
		return err
	}
	port := env("PORT", "8080")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("PORT must be between 1 and 65535")
	}
	srv := &http.Server{Addr: net.JoinHostPort("0.0.0.0", port), Handler: observability.Middleware(handler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	failed := make(chan error, 1)
	go func() {
		slog.Info("documentation server listening", "port", portNumber, "version", version)
		failed <- srv.ListenAndServe()
	}()
	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		grace, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(grace); err != nil {
			_ = srv.Close()
			return err
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
