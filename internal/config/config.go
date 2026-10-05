// Package config 负责运行时配置：读取环境变量、提供默认值，并在真正需要时
// 对必填项（R2 凭据）做校验。
//
// 对应参考项目 src/lib/server/config.ts 与 src/env.ts。刻意不在启动阶段抛错：
// 缺少 R2 凭据时页面仍可访问，只有调用解析接口时才返回明确的配置错误。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"resparse/internal/apperr"
)

// 日志级别字符串。
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

const defaultMaxFileBytes int64 = 2 * 1024 * 1024 * 1024 // 2 GiB

// Config 应用配置。
type Config struct {
	// R2（原始值，缺失时在 ResolveR2 中校验）
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Endpoint        string
	Region          string

	// 服务
	APIKey string
	Host   string
	Port   int

	// ffprobe
	FfprobePath  string
	ProbeTimeout time.Duration

	// 其他
	LogLevel      string
	MaxFileBytes  int64
	TempDirPrefix string
}

// R2Config 解析后的 R2 配置。
type R2Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

// Load 从环境变量加载配置并填充默认值。
func Load() *Config {
	return &Config{
		AccountID:       trimmed(os.Getenv("R2_ACCOUNT_ID")),
		AccessKeyID:     trimmed(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: trimmed(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Bucket:          trimmed(os.Getenv("R2_BUCKET")),
		Endpoint:        trimmed(os.Getenv("R2_ENDPOINT")),
		Region:          firstNonEmpty(trimmed(os.Getenv("R2_REGION")), "auto"),

		APIKey: trimmed(os.Getenv("API_KEY")),
		Host:   firstNonEmpty(trimmed(os.Getenv("HOST")), "0.0.0.0"),
		Port:   positiveInt(os.Getenv("PORT"), 3000),

		FfprobePath:  firstNonEmpty(trimmed(os.Getenv("FFPROBE_PATH")), "ffprobe"),
		ProbeTimeout: positiveDuration(os.Getenv("PROBE_TIMEOUT_MS"), 30*time.Second),

		LogLevel:      normalizeLogLevel(os.Getenv("LOG_LEVEL")),
		MaxFileBytes:  positiveInt64(os.Getenv("MAX_FILE_BYTES"), defaultMaxFileBytes),
		TempDirPrefix: firstNonEmpty(trimmed(os.Getenv("TEMP_DIR_PREFIX")), "metaparse-"),
	}
}

// ResolveR2 校验并返回 R2 配置；缺失必要项时返回 config_missing 错误。
func (c *Config) ResolveR2() (R2Config, error) {
	var missing []string
	if c.AccountID == "" {
		missing = append(missing, "R2_ACCOUNT_ID")
	}
	if c.Bucket == "" {
		missing = append(missing, "R2_BUCKET")
	}
	if c.AccessKeyID == "" {
		missing = append(missing, "R2_ACCESS_KEY_ID")
	}
	if c.SecretAccessKey == "" {
		missing = append(missing, "R2_SECRET_ACCESS_KEY")
	}
	if len(missing) > 0 {
		return R2Config{}, apperr.New(
			"config_missing",
			"缺少 R2 配置："+strings.Join(missing, ", "),
			500,
			map[string]any{"missing": missing},
		)
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = ResolveEndpoint(c.AccountID)
	}

	return R2Config{
		Endpoint:        endpoint,
		Region:          c.Region,
		Bucket:          c.Bucket,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
	}, nil
}

// HasR2 报告 R2 是否已配置（用于健康检查展示）。
func (c *Config) HasR2() bool {
	return c.AccountID != "" && c.Bucket != "" && c.AccessKeyID != "" && c.SecretAccessKey != ""
}

// ResolveEndpoint 由账号 ID 生成默认 endpoint：https://<account>.r2.cloudflarestorage.com。
func ResolveEndpoint(accountID string) string {
	base := accountID
	base = strings.TrimPrefix(base, "https://")
	base = strings.TrimPrefix(base, "http://")
	base = strings.TrimRight(base, "/")
	return "https://" + base + ".r2.cloudflarestorage.com"
}

func trimmed(v string) string { return strings.TrimSpace(v) }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeLogLevel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case LevelDebug:
		return LevelDebug
	case LevelWarn:
		return LevelWarn
	case LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

// positiveDuration 将毫秒字符串解析为 time.Duration；非法或 <=0 时回退。
func positiveDuration(v string, fallback time.Duration) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Millisecond
}

func positiveInt(v string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func positiveInt64(v string, fallback int64) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
