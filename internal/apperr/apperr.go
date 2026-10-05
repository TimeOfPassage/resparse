// Package apperr 定义带 HTTP 状态语义的应用级错误。
//
// 对应参考项目 src/lib/server/errors.ts 中的 AppError。
// API 路由据此返回统一的结构化错误响应。
package apperr

import "errors"

// AppError 是携带错误码与 HTTP 状态的应用级错误。
type AppError struct {
	Code    string
	Message string
	Status  int
	Details any
}

// Error 实现 error 接口。
func (e *AppError) Error() string { return e.Message }

// New 构造一个 AppError；status 为 0 时回退为 500。
func New(code, message string, status int, details any) *AppError {
	if status == 0 {
		status = 500
	}
	return &AppError{Code: code, Message: message, Status: status, Details: details}
}

// As 从错误链中提取 *AppError。
func As(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
