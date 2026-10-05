package api

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"resparse/internal/apperr"
	"resparse/internal/config"
	"resparse/internal/logging"
)

// checkAuth 校验 API Key；未配置 API_KEY 时直接放行。
func checkAuth(cfg *config.Config, r *http.Request) error {
	if cfg.APIKey == "" {
		return nil
	}
	auth := r.Header.Get("Authorization")
	bearer := ""
	if len(auth) >= 7 && strings.EqualFold(auth[:7], "bearer ") {
		bearer = strings.TrimSpace(auth[7:])
	}
	headerKey := strings.TrimSpace(r.Header.Get("x-api-key"))
	if constantEquals(bearer, cfg.APIKey) || constantEquals(headerKey, cfg.APIKey) {
		return nil
	}
	return apperr.New("unauthorized", "认证失败：缺少或错误的 API Key", 401, nil)
}

func constantEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// writeJSON 输出 JSON 响应，统一设置 no-store。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError 将未知错误转换为统一的 JSON 错误响应。
func writeError(w http.ResponseWriter, err error) {
	if ae, ok := apperr.As(err); ok {
		body := map[string]any{"code": ae.Code, "message": ae.Message}
		if ae.Details != nil {
			body["details"] = ae.Details
		}
		writeJSON(w, ae.Status, map[string]any{"error": body})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": map[string]any{"code": "internal_error", "message": err.Error()},
	})
}

// requireKey 校验并返回非空字符串 key。
func requireKey(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", apperr.New("invalid_key", "缺少参数 key", 400, nil)
	}
	return value, nil
}

// parseRequestBody 解析请求体为字段对象：JSON 取字段，纯文本视为 {key: <text>}。
func parseRequestBody(r *http.Request) map[string]any {
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			return nil
		}
		return m
	}
	b, _ := io.ReadAll(r.Body)
	text := strings.TrimSpace(string(b))
	if text == "" {
		return nil
	}
	return map[string]any{"key": text}
}

// recorder 缓冲响应，便于中间件在写回前读取状态码与错误码。
type recorder struct {
	header http.Header
	status int
	buf    bytes.Buffer
	wrote  bool
}

func newRecorder() *recorder { return &recorder{header: make(http.Header)} }

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.buf.Write(b)
}

// withRequestLog 为处理器补充统一日志与 x-request-id 响应头。
//
// 成功（<400）记 info，4xx 记 warn，5xx 记 error，均包含耗时；
// 4xx/5xx 会尝试从响应体提取 error.code 一并记录。
func (s *Server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := newRequestID()

		base := map[string]any{
			"requestId": requestID,
			"method":    r.Method,
			"path":      r.URL.Path,
			"query":     queryMap(r.URL.Query()),
		}
		if ip := clientIP(r); ip != "" {
			base["ip"] = ip
		}
		if ua := r.Header.Get("User-Agent"); ua != "" {
			base["userAgent"] = ua
		}

		logging.Info("http.request", base)

		rec := newRecorder()
		func() {
			defer func() {
				if p := recover(); p != nil {
					logging.Error("http.unhandled", merge(base, map[string]any{
						"durationMs": time.Since(started).Milliseconds(),
						"error":      fmt.Sprintf("%v", p),
					}))
					if !rec.wrote {
						writeError(rec, apperr.New("internal_error", "服务器内部错误", 500, nil))
					}
				}
			}()
			next.ServeHTTP(rec, r)
		}()

		durationMs := time.Since(started).Milliseconds()

		var code string
		if rec.status >= 400 {
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(rec.buf.Bytes(), &body) == nil {
				code = body.Error.Code
			}
		}

		fields := merge(base, map[string]any{
			"status":     rec.status,
			"durationMs": durationMs,
		})
		if code != "" {
			fields["code"] = code
		}

		switch {
		case rec.status >= 500:
			logging.Error("http.response", fields)
		case rec.status >= 400:
			logging.Warn("http.response", fields)
		default:
			logging.Info("http.response", fields)
		}

		for k, vs := range rec.header {
			w.Header()[k] = vs
		}
		w.Header().Set("x-request-id", requestID)
		w.WriteHeader(rec.status)
		_, _ = w.Write(rec.buf.Bytes())
	})
}

func merge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func queryMap(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for k, vals := range values {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

// clientIP 提取客户端 IP（优先 X-Forwarded-For，其次 X-Real-Ip）。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-Ip"); xr != "" {
		return strings.TrimSpace(xr)
	}
	return ""
}

// newRequestID 生成 UUID v4 作为请求 ID。
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
