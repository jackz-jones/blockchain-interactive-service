// Package errcode 提供项目统一的业务错误码定义与 BizError 类型
//
// 设计目标：
//  1. 用统一的 code + message 表达业务错误，替代散落在 logic 层的字符串拼接
//  2. 与 HTTP 状态码解耦：一个 4xx/5xx 状态可以对应多个业务错误码
//  3. 保留可选的 http status，方便中间件/handler 决定响应码
//  4. 与 go-zero 的 errorx.CodeError 风格保持兼容，可被 rest 框架识别
package errcode

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 业务错误码（int）
// 编码规约：
//   -   0        成功
//   -   1xxx     公共通用错误
//   -   2xxx     认证 / 权限相关
//   -   3xxx     配额 / 计费相关
//   -   4xxx     配置 / 资源相关
//   -   5xxx     SDK / 链交互相关
//   -   9xxx     内部错误 / 未分类
type Code int

// 通用错误码
const (
	OK Code = 0

	// 1xxx 通用
	InvalidParam    Code = 1001
	NotFound        Code = 1002
	Conflict        Code = 1003
	TooManyRequests Code = 1004

	// 2xxx 认证与权限
	Unauthorized     Code = 2001
	InvalidAPIKey    Code = 2002
	APIKeyExpired    Code = 2003
	APIKeyRevoked    Code = 2004
	Forbidden        Code = 2005
	IPNotAllowed     Code = 2006
	TenantDisabled   Code = 2007

	// 3xxx 配额 / 计费
	QuotaExceededDaily   Code = 3001
	QuotaExceededMonth   Code = 3002
	QuotaThrottled       Code = 3003
	BillingUnavailable   Code = 3004

	// 4xxx 配置 / 资源
	ChainConfigNotFound    Code = 4001
	ContractConfigNotFound Code = 4002
	AlreadySubscribed      Code = 4003
	NotSubscribed          Code = 4004

	// 5xxx SDK / 链交互
	SDKInitFailed        Code = 5001
	SDKCallFailed        Code = 5002
	ChainRPCUnavailable  Code = 5003
	InvalidChainPayload  Code = 5004

	// 9xxx 内部
	InternalError Code = 9001
)

// BizError 业务错误
// 遵循 error 接口，可在 logic 层用 errors.As/Is 判定
type BizError struct {
	Code       Code
	Message    string
	HTTPStatus int   // 可选：0 表示由上层根据 Code 自行推断
	Cause      error // 可选：底层原因，仅用于日志，不下发给客户端
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持 errors.Is/As 链式判断
func (e *BizError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New 构造 BizError（无底层原因）
func New(code Code, msg string) *BizError {
	return &BizError{Code: code, Message: msg}
}

// Newf 构造 BizError（格式化）
func Newf(code Code, format string, args ...any) *BizError {
	return &BizError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap 用 BizError 包装底层错误
func Wrap(code Code, msg string, cause error) *BizError {
	return &BizError{Code: code, Message: msg, Cause: cause}
}

// AsBizError 从任意 error 中提取 BizError；若不是则返回 nil, false
func AsBizError(err error) (*BizError, bool) {
	if err == nil {
		return nil, false
	}
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

// HTTPStatusOf 根据 Code 推断 HTTP 状态码；BizError 已显式设置则优先使用
func HTTPStatusOf(err error) int {
	if be, ok := AsBizError(err); ok {
		if be.HTTPStatus > 0 {
			return be.HTTPStatus
		}
		return defaultHTTPStatus(be.Code)
	}
	if err != nil {
		return http.StatusInternalServerError
	}
	return http.StatusOK
}

// defaultHTTPStatus 各错误码对应的默认 HTTP 状态码
func defaultHTTPStatus(c Code) int {
	switch {
	case c == OK:
		return http.StatusOK
	case c == InvalidParam:
		return http.StatusBadRequest
	case c == NotFound || c == ChainConfigNotFound || c == ContractConfigNotFound || c == NotSubscribed:
		return http.StatusNotFound
	case c == Conflict || c == AlreadySubscribed:
		return http.StatusConflict
	case c == TooManyRequests || c == QuotaThrottled:
		return http.StatusTooManyRequests
	case c == Unauthorized || c == InvalidAPIKey || c == APIKeyExpired || c == APIKeyRevoked:
		return http.StatusUnauthorized
	case c == Forbidden || c == IPNotAllowed || c == TenantDisabled:
		return http.StatusForbidden
	case c == QuotaExceededDaily || c == QuotaExceededMonth:
		return http.StatusPaymentRequired
	case c >= 5000 && c < 6000:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
