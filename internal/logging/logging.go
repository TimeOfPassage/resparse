// Package logging 提供极简结构化 JSON 日志。
//
// 对应参考项目 src/lib/server/logger.ts：单行 JSON 输出到 stdout/stderr
// （warn/error 走 stderr），便于容器日志采集按字段检索。级别由 LOG_LEVEL 控制。
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	// LevelDebug 调试级别。
	LevelDebug Level = iota
	// LevelInfo 信息级别。
	LevelInfo
	// LevelWarn 警告级别。
	LevelWarn
	// LevelError 错误级别。
	LevelError
)

const tsFormat = "2006-01-02T15:04:05.000Z"

// ParseLevel 解析日志级别字符串，非法值回退为 info。
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger 结构化日志器。
type Logger struct {
	level  Level
	out    io.Writer
	errOut io.Writer
	mu     sync.Mutex
	now    func() time.Time
}

var std = &Logger{level: LevelInfo, out: os.Stdout, errOut: os.Stderr, now: time.Now}

// SetLevel 设置全局日志级别。
func SetLevel(l Level) { std.level = l }

// SetOutput 设置全局日志输出目标（测试用）。
func SetOutput(out, errOut io.Writer) {
	std.out = out
	std.errOut = errOut
}

// Debug 输出 debug 级日志。
func Debug(msg string, fields map[string]any) { std.emit(LevelDebug, "debug", msg, fields) }

// Info 输出 info 级日志。
func Info(msg string, fields map[string]any) { std.emit(LevelInfo, "info", msg, fields) }

// Warn 输出 warn 级日志。
func Warn(msg string, fields map[string]any) { std.emit(LevelWarn, "warn", msg, fields) }

// Error 输出 error 级日志。
func Error(msg string, fields map[string]any) { std.emit(LevelError, "error", msg, fields) }

func (l *Logger) emit(lv Level, lvName, msg string, fields map[string]any) {
	if lv < l.level {
		return
	}
	rec := make(map[string]any, len(fields)+3)
	for k, v := range fields {
		rec[k] = normalize(v)
	}
	rec["level"] = lvName
	rec["msg"] = msg
	rec["ts"] = l.now().UTC().Format(tsFormat)

	line, err := json.Marshal(rec)
	if err != nil {
		// 循环引用等极端情况下的兜底，保证日志本身不抛错。
		line, _ = json.Marshal(map[string]any{
			"level":           lvName,
			"msg":             msg,
			"_serializeError": true,
		})
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if lv >= LevelWarn {
		_, _ = l.errOut.Write(line)
	} else {
		_, _ = l.out.Write(line)
	}
}

// normalize 将日志字段中的特殊类型转换为可序列化形式。
func normalize(v any) any {
	switch t := v.(type) {
	case error:
		return map[string]any{"name": fmt.Sprintf("%T", t), "message": t.Error()}
	case time.Time:
		return t.UTC().Format(tsFormat)
	default:
		return v
	}
}
