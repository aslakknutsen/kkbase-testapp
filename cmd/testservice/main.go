package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aslakknutsen/kkbase/testapp/pkg/service"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/behavior"
	grpcserver "github.com/aslakknutsen/kkbase/testapp/pkg/service/grpc"
	httpserver "github.com/aslakknutsen/kkbase/testapp/pkg/service/http"
	"github.com/aslakknutsen/kkbase/testapp/pkg/service/telemetry"
	pb "github.com/aslakknutsen/kkbase/testapp/proto/testservice"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := service.LoadConfigFromEnv()

	// Initialize telemetry
	tel, err := telemetry.InitTelemetry(
		cfg.Name,
		cfg.Namespace,
		cfg.LogLevel,
		cfg.OTELEndpoint,
		cfg,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize telemetry: %v\n", err)
		os.Exit(1)
	}

	tel.Logger.Info("Starting testservice",
		zap.String("name", cfg.Name),
		zap.String("version", cfg.Version),
		zap.Int("http_port", cfg.HTTPPort),
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("metrics_port", cfg.MetricsPort),
		zap.Int("upstreams", len(cfg.Upstreams)),
	)

	// Check for CRASH_ON_FILE_CONTENT configuration
	if crashOnFileContent := os.Getenv("CRASH_ON_FILE_CONTENT"); crashOnFileContent != "" {
		tel.Logger.Info("Checking for invalid config file content", zap.String("config", crashOnFileContent))
		checkCrashOnFileContent(crashOnFileContent, tel)
	}

	// Create servers
	httpSrv := httpserver.NewServer(cfg, tel)
	grpcSrv := grpcserver.NewServer(cfg, tel)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Setup HTTP handler
	httpMux := http.NewServeMux()
	httpMux.Handle("/", httpSrv)
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	httpMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Handler: httpMux,
	}

	// Setup gRPC server with Prometheus interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)
	pb.RegisterTestServiceServer(grpcServer, grpcSrv)

	// Initialize gRPC metrics
	grpc_prometheus.Register(grpcServer)

	// Determine which port configuration to use
	// If HTTP and gRPC ports are the same, use cmux for multiplexing
	// Otherwise, start them on separate ports (backward compatibility)
	if cfg.HTTPPort == cfg.GRPCPort {
		// Unified port mode: use cmux to multiplex HTTP and gRPC on same port
		tel.Logger.Info("Starting unified HTTP/gRPC server with cmux",
			zap.Int("port", cfg.HTTPPort))

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HTTPPort))
		if err != nil {
			tel.Logger.Fatal("Failed to create listener", zap.Error(err))
		}

		// Create cmux multiplexer
		mux := cmux.New(listener)

		// Match HTTP/1.x requests
		httpListener := mux.Match(cmux.HTTP1Fast())

		// Match HTTP/2 (gRPC) requests
		grpcListener := mux.Match(cmux.HTTP2())

		// Start HTTP server
		go func() {
			tel.Logger.Info("HTTP server starting on unified port", zap.Int("port", cfg.HTTPPort))
			if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
				tel.Logger.Fatal("HTTP server failed", zap.Error(err))
			}
		}()

		// Start gRPC server
		go func() {
			tel.Logger.Info("gRPC server starting on unified port", zap.Int("port", cfg.HTTPPort))
			if err := grpcServer.Serve(grpcListener); err != nil {
				tel.Logger.Fatal("gRPC server failed", zap.Error(err))
			}
		}()

		// Start the multiplexer
		go func() {
			if err := mux.Serve(); err != nil {
				tel.Logger.Fatal("cmux server failed", zap.Error(err))
			}
		}()
	} else {
		// Separate port mode: traditional setup
		tel.Logger.Info("Starting HTTP and gRPC servers on separate ports",
			zap.Int("http_port", cfg.HTTPPort),
			zap.Int("grpc_port", cfg.GRPCPort))

		// Start HTTP server
		httpServer.Addr = fmt.Sprintf(":%d", cfg.HTTPPort)
		go func() {
			tel.Logger.Info("HTTP server starting", zap.Int("port", cfg.HTTPPort))
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				tel.Logger.Fatal("HTTP server failed", zap.Error(err))
			}
		}()

		// Start gRPC server
		grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			tel.Logger.Fatal("Failed to listen for gRPC", zap.Error(err))
		}

		go func() {
			tel.Logger.Info("gRPC server starting", zap.Int("port", cfg.GRPCPort))
			if err := grpcServer.Serve(grpcListener); err != nil {
				tel.Logger.Fatal("gRPC server failed", zap.Error(err))
			}
		}()
	}

	// Start metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: metricsMux,
	}

	go func() {
		tel.Logger.Info("Metrics server starting", zap.Int("port", cfg.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			tel.Logger.Error("Metrics server failed", zap.Error(err))
		}
	}()

	tel.Logger.Info("All servers started successfully")

	// Wait for shutdown signal
	<-sigChan
	tel.Logger.Info("Shutdown signal received, gracefully shutting down...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		tel.Logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	grpcServer.GracefulStop()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		tel.Logger.Error("Metrics server shutdown error", zap.Error(err))
	}

	tel.Logger.Info("Shutdown complete")
}

// checkCrashOnFileContent checks for invalid content in config files and crashes if found
// Format: /path/to/file:invalid1,invalid2|/other/file:bad
func checkCrashOnFileContent(config string, tel *telemetry.Telemetry) {
	// Split by pipe to handle multiple file checks
	fileChecks := strings.Split(config, "|")
	
	for _, check := range fileChecks {
		check = strings.TrimSpace(check)
		if check == "" {
			continue
		}

		// Parse using the same format as crash-if-file behavior
		crashBehavior, err := behavior.Parse(fmt.Sprintf("crash-if-file=%s", check))
		if err != nil {
			tel.Logger.Warn("Failed to parse CRASH_ON_FILE_CONTENT entry",
				zap.String("entry", check),
				zap.Error(err))
			continue
		}

		if crashBehavior.CrashIfFile == nil {
			continue
		}

		// Check if file contains invalid content
		shouldCrash, matched, msg := crashBehavior.ShouldCrashOnFile()
		if shouldCrash {
			tel.Logger.Fatal("Config file contains invalid content - crashing as configured",
				zap.String("file", crashBehavior.CrashIfFile.FilePath),
				zap.String("matched_content", matched),
				zap.String("message", msg))
			os.Exit(1)
		}
	}
}
