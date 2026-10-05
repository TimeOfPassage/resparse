// Package api 提供 HTTP 路由与处理器。
//
// 对应参考项目的 src/routes/api/*。对外提供：
//
//	GET  /api/metadata?key=<r2-object-key>
//	POST /api/metadata   body: {"key":"..."} 或纯文本 key
//	GET  /api/health
//	GET  /               演示页
package api

import (
	"net/http"
	"time"

	"resparse/internal/config"
	"resparse/internal/ffprobe"
	"resparse/internal/metadata"
	"resparse/internal/webui"
)

// Server 聚合依赖并提供路由。
type Server struct {
	cfg       *config.Config
	svc       *metadata.Service
	runner    *ffprobe.Runner
	startedAt time.Time
}

// NewServer 构造 Server。
func NewServer(cfg *config.Config, svc *metadata.Service, runner *ffprobe.Runner) *Server {
	return &Server{cfg: cfg, svc: svc, runner: runner, startedAt: time.Now()}
}

// Routes 返回 HTTP 处理器。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	// 健康检查不接入请求日志（避免容器探测产生日志噪音）。
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.Handle("GET /api/metadata", s.withRequestLog(http.HandlerFunc(s.handleGetMetadata)))
	mux.Handle("POST /api/metadata", s.withRequestLog(http.HandlerFunc(s.handlePostMetadata)))
	mux.HandleFunc("GET /", s.handleIndex)
	return mux
}

func (s *Server) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if err := checkAuth(s.cfg, r); err != nil {
		writeError(w, err)
		return
	}
	key, err := requireKey(r.URL.Query().Get("key"))
	if err != nil {
		writeError(w, err)
		return
	}
	s.respondMetadata(w, r, key)
}

func (s *Server) handlePostMetadata(w http.ResponseWriter, r *http.Request) {
	if err := checkAuth(s.cfg, r); err != nil {
		writeError(w, err)
		return
	}
	body := parseRequestBody(r)
	keyStr := ""
	if body != nil {
		if v, ok := body["key"].(string); ok {
			keyStr = v
		}
	}
	key, err := requireKey(keyStr)
	if err != nil {
		writeError(w, err)
		return
	}
	s.respondMetadata(w, r, key)
}

func (s *Server) respondMetadata(w http.ResponseWriter, r *http.Request, key string) {
	result, err := s.svc.Parse(r.Context(), key)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleHealth 报告进程存活、ffprobe 可用性与 R2 配置状态。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	version := s.runner.Version(r.Context())
	ffprobeOK := version != nil
	r2Configured := s.cfg.HasR2()
	healthy := ffprobeOK

	status := "ok"
	switch {
	case !healthy:
		status = "unhealthy"
	case !r2Configured:
		status = "degraded"
	}

	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status": status,
		"checks": map[string]any{
			"ffprobe": map[string]any{
				"ok":      ffprobeOK,
				"path":    s.runner.Path(),
				"version": version,
			},
			"r2": map[string]any{"configured": r2Configured},
		},
		"limits": map[string]any{
			"maxFileBytes":   s.cfg.MaxFileBytes,
			"probeTimeoutMs": s.cfg.ProbeTimeout.Milliseconds(),
		},
		"uptimeSeconds": int64(time.Since(s.startedAt).Seconds()),
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(webui.Index())
}
