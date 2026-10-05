// Package ffprobe 封装 ffprobe 子进程调用，以 JSON 形式输出
// format / streams / chapters。
//
// 对应参考项目 src/lib/server/ffprobe.ts。
package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"resparse/internal/apperr"
	"resparse/internal/execx"
)

// Output 对应 `ffprobe -print_format json` 的输出。
//
// Streams / Format / Chapters 以原始 map 保留，以便透传未归一化字段。
type Output struct {
	Streams  []map[string]any `json:"streams"`
	Format   map[string]any   `json:"format"`
	Chapters []map[string]any `json:"chapters"`
	Error    *ProbeError      `json:"error"`
}

// ProbeError ffprobe 在 JSON 中返回的错误信息。
type ProbeError struct {
	Code   int    `json:"code"`
	String string `json:"string"`
}

// ProbeResult 单次探测结果。
type ProbeResult struct {
	Data       Output
	Raw        []byte
	DurationMs int64
}

// Runner 复用 ffprobe 路径与超时配置，并缓存版本号。
type Runner struct {
	path    string
	timeout time.Duration

	versionOnce sync.Once
	version     *string
}

// NewRunner 构造 Runner。
func NewRunner(path string, timeout time.Duration) *Runner {
	return &Runner{path: path, timeout: timeout}
}

// Path 返回 ffprobe 可执行文件路径。
func (r *Runner) Path() string { return r.path }

// Run 对本地文件执行 ffprobe。
//
// 探测失败（非 0 退出码）返回 probe_failed(422)，供调用方降级为「仅返回 source」；
// 超时、找不到 ffprobe 等仍按对应错误返回。
func (r *Runner) Run(ctx context.Context, filePath string) (*ProbeResult, error) {
	started := time.Now()
	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		filePath,
	}

	res, err := execx.Run(ctx, r.path, args, execx.Options{
		Timeout:        r.timeout,
		TimeoutCode:    "probe_timeout",
		NotFoundCode:   "ffprobe_not_found",
		SpawnErrorCode: "ffprobe_spawn_failed",
		Label:          "ffprobe",
	})
	if err != nil {
		return nil, err
	}
	durationMs := time.Since(started).Milliseconds()

	if res.Code != 0 {
		msg := res.Stderr
		if msg == "" {
			msg = fmt.Sprintf("退出码 %d", res.Code)
		}
		return nil, apperr.New("probe_failed", "ffprobe 解析失败："+msg, 422, nil)
	}

	var out Output
	if err := json.Unmarshal(res.Stdout, &out); err != nil {
		return nil, apperr.New("probe_parse_failed", "ffprobe 输出不是合法 JSON", 500,
			map[string]any{"stdout": truncate(string(res.Stdout), 2000)})
	}
	if out.Error != nil {
		return nil, apperr.New("probe_failed", "ffprobe 解析失败："+out.Error.String, 422, nil)
	}

	return &ProbeResult{Data: out, Raw: res.Stdout, DurationMs: durationMs}, nil
}

// Version 获取 ffprobe 版本号（首次调用后缓存）。
func (r *Runner) Version(ctx context.Context) *string {
	r.versionOnce.Do(func() {
		res, err := execx.Run(ctx, r.path, []string{"-version"}, execx.Options{
			Timeout:        5 * time.Second,
			NotFoundCode:   "ffprobe_not_found",
			SpawnErrorCode: "ffprobe_spawn_failed",
			Label:          "ffprobe",
		})
		if err != nil || res.Code != 0 {
			return
		}
		line := strings.TrimSpace(string(res.Stdout))
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			r.version = &line
		}
	})
	return r.version
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
