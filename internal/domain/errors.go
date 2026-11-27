package domain

import "errors"

var (
	ErrInvalidArgument = errors.New("invalid_argument")
	ErrUnavailable     = errors.New("unavailable")
	ErrRateLimited     = errors.New("rate_limited")
)

// Error 的公开描述不会泄露 SQL、连接串或模型提供商的响应体。
// 多错误解包保留领域分类与 context 取消语义，调用方可使用 errors.Is。
type Error struct {
	Kind    error
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Kind.Error() + ": " + e.Message }
func (e *Error) Unwrap() []error {
	if e.Cause == nil {
		return []error{e.Kind}
	}
	return []error{e.Kind, e.Cause}
}
func Invalid(message string) error { return &Error{Kind: ErrInvalidArgument, Message: message} }
func Unavailable(cause error) error {
	return &Error{Kind: ErrUnavailable, Message: "依赖服务不可用", Cause: cause}
}
