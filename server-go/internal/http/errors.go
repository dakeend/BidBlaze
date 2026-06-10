package http

import (
	"errors"
	"fmt"
	"net/http"
)

type Code int

const (
	CodeOK Code = 0

	CodeInvalidParam Code = 1001
	CodeUnauthorized Code = 1002
	CodeForbidden    Code = 1003
	CodeTooFrequent  Code = 1004
	CodeIdempotent   Code = 1005

	CodeAuctionNotFound Code = 2001
	CodeAuctionPending  Code = 2002
	CodeAuctionEnded    Code = 2003
	CodeAuctionCancel   Code = 2004

	CodeBidTooLow      Code = 2101
	CodeBidOverCeiling Code = 2102
	CodeBidConflict    Code = 2103

	CodeInternal Code = 9999
)

// DevMode 控制是否在非 AppError 中暴露真实错误信息。
// 仅在 dev 环境下启用，便于调试。
var DevMode bool

type AppError struct {
	Code       Code
	HTTPStatus int
	Message    string
	Data       any
	Err        error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewErrorData(code Code, status int, message string, data any) *AppError {
	if message == "" {
		message = "error"
	}
	return &AppError{
		Code:       code,
		HTTPStatus: status,
		Message:    message,
		Data:       data,
	}
}

func AsAppError(err error) *AppError {
	if err == nil {
		return NewErrorData(CodeInternal, http.StatusInternalServerError, "internal error", nil)
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		if DevMode && appErr.Err != nil {
			// dev 环境下将原始错误详情拼入 message，方便前端直接看到原因。
			appErr = &AppError{
				Code:       appErr.Code,
				HTTPStatus: appErr.HTTPStatus,
				Message:    fmt.Sprintf("%s [dev: %v]", appErr.Message, appErr.Err),
				Data:       appErr.Data,
				Err:        appErr.Err,
			}
		}
		return appErr
	}
	msg := "internal error"
	if DevMode {
		msg = fmt.Sprintf("internal error [dev: %v]", err)
	}
	return &AppError{
		Code:       CodeInternal,
		HTTPStatus: http.StatusInternalServerError,
		Message:    msg,
		Err:        err,
	}
}

func InvalidParam(message string) *AppError {
	return NewErrorData(CodeInvalidParam, http.StatusBadRequest, message, nil)
}

func Unauthorized() *AppError {
	return NewErrorData(CodeUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
}

func Forbidden() *AppError {
	return NewErrorData(CodeForbidden, http.StatusForbidden, "forbidden", nil)
}

func IdempotencyConflict() *AppError {
	return NewErrorData(CodeIdempotent, http.StatusConflict, "idempotency key conflict", nil)
}

func AuctionNotFound() *AppError {
	return NewErrorData(CodeAuctionNotFound, http.StatusNotFound, "auction not found", nil)
}

func AuctionNotPending() *AppError {
	return NewErrorData(CodeAuctionPending, http.StatusOK, "auction is not pending", nil)
}

func AuctionEnded() *AppError {
	return NewErrorData(CodeAuctionEnded, http.StatusOK, "auction ended", nil)
}

func AuctionCancelled() *AppError {
	return NewErrorData(CodeAuctionCancel, http.StatusOK, "auction cancelled", nil)
}

func BidConflict() *AppError {
	return NewErrorData(CodeBidConflict, http.StatusConflict, "bid conflict", nil)
}

func SystemProtect(err error) *AppError {
	return &AppError{
		Code:       CodeTooFrequent,
		HTTPStatus: http.StatusTooManyRequests,
		Message:    "request too frequent",
		Err:        err,
	}
}
