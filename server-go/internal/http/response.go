package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

type Envelope struct {
	Code Code `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Code: CodeOK,
		Msg:  "ok",
		Data: data,
	})
}

func Fail(c *gin.Context, err error) {
	appErr := AsAppError(err)
	c.JSON(appErr.HTTPStatus, Envelope{
		Code: appErr.Code,
		Msg:  appErr.Message,
		Data: appErr.Data,
	})
}
