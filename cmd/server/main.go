// Command server 是 MetaParse（Go 版）的入口。
//
// 用法：
//
//	server                启动 HTTP 服务
//	server healthcheck    以 HTTP 探测 /api/health，供容器健康检查使用
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"resparse/internal/api"
	"resparse/internal/config"
	"resparse/internal/ffprobe"
	"resparse/internal/logging"
	"resparse/internal/metadata"
)

func main() {
	cfg := config.Load()
	logging.SetLevel(logging.ParseLevel(cfg.LogLevel))

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(cfg))
	}

	runner := ffprobe.NewRunner(cfg.FfprobePath, cfg.ProbeTimeout)
	svc := metadata.NewService(cfg, runner)
	srv := api.NewServer(cfg, svc, runner)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logging.Info("server.started", map[string]any{
			"addr":         addr,
			"logLevel":     cfg.LogLevel,
			"r2Configured": cfg.HasR2(),
		})
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		logging.Error("server.error", map[string]any{"error": err.Error()})
		os.Exit(1)
	case sig := <-stop:
		logging.Info("server.stopping", map[string]any{"signal": sig.String()})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logging.Error("server.shutdown_error", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}

// healthcheck 请求本地 /api/health，返回进程退出码。
func healthcheck(cfg *config.Config) int {
	host := cfg.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d/api/health", host, cfg.Port)

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "healthcheck failed: HTTP %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
