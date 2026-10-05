// Package execx 封装子进程执行：收集 stdout/stderr，支持超时强杀与
// 可执行文件缺失的错误码映射。
//
// 对应参考项目 src/lib/server/exec.ts。
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"resparse/internal/apperr"
)

// Result 子进程执行结果。
type Result struct {
	Stdout []byte
	Stderr string
	Code   int
}

// Options 执行选项。
type Options struct {
	// Timeout 超时时间，超时后强杀并以 TimeoutCode 抛错。
	Timeout time.Duration
	// TimeoutCode 超时错误码，默认 process_timeout。
	TimeoutCode string
	// NotFoundCode 可执行文件缺失（ENOENT）错误码，默认 executable_not_found。
	NotFoundCode string
	// SpawnErrorCode 其他启动失败错误码，默认 process_spawn_failed。
	SpawnErrorCode string
	// Label 日志/错误信息中的可读名称，默认取命令名。
	Label string
}

// Run 执行命令并返回收集到的输出；失败时返回带 HTTP 语义的 *apperr.AppError。
//
// 命令以非 0 退出码结束时返回 Result 且 error 为 nil，由调用方根据 Code 处理。
func Run(ctx context.Context, command string, args []string, opts Options) (Result, error) {
	timeoutCode := defaultStr(opts.TimeoutCode, "process_timeout")
	notFoundCode := defaultStr(opts.NotFoundCode, "executable_not_found")
	spawnErrorCode := defaultStr(opts.SpawnErrorCode, "process_spawn_failed")
	label := defaultStr(opts.Label, command)

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return Result{}, apperr.New(
				timeoutCode,
				fmt.Sprintf("%s 执行超时（>%dms）", label, opts.Timeout.Milliseconds()),
				504, nil,
			)
		}
		if errors.Is(err, exec.ErrNotFound) {
			return Result{}, apperr.New(
				notFoundCode,
				fmt.Sprintf("找不到可执行文件（%s），请确认已安装或设置路径", command),
				500, nil,
			)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Result{
				Stdout: stdout.Bytes(),
				Stderr: strings.TrimSpace(stderr.String()),
				Code:   exitErr.ExitCode(),
			}, nil
		}
		return Result{}, apperr.New(
			spawnErrorCode,
			fmt.Sprintf("启动 %s 失败：%v", label, err),
			500, nil,
		)
	}

	return Result{
		Stdout: stdout.Bytes(),
		Stderr: strings.TrimSpace(stderr.String()),
		Code:   0,
	}, nil
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
