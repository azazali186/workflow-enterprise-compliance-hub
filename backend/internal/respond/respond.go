// Package respond centralizes HTTP JSON responses so all handlers share one
// consistent envelope shape.
package respond

import (
	"context"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Body is the standard success envelope.
type Body struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrorBody is the standard error envelope.
type ErrorBody struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func OK(c *app.RequestContext, data any) {
	c.JSON(consts.StatusOK, Body{Success: true, Data: data})
}

func OKWithMeta(c *app.RequestContext, data, meta any) {
	c.JSON(consts.StatusOK, Body{Success: true, Data: data, Meta: meta})
}

func Created(c *app.RequestContext, data any) {
	c.JSON(consts.StatusCreated, Body{Success: true, Data: data})
}

func NoContent(c *app.RequestContext) {
	c.SetStatusCode(consts.StatusNoContent)
}

func Error(c *app.RequestContext, status int, code string, err error) {
	logError(c, err)
	c.JSON(status, ErrorBody{Success: false, Code: code, Error: errMsg(err)})
}

func BadRequest(c *app.RequestContext, code string, err error) {
	Error(c, consts.StatusBadRequest, code, err)
}

func NotFound(c *app.RequestContext, code string, err error) {
	Error(c, consts.StatusNotFound, code, err)
}

func Conflict(c *app.RequestContext, code string, err error) {
	Error(c, consts.StatusConflict, code, err)
}

func Unauthorized(c *app.RequestContext) {
	c.JSON(consts.StatusUnauthorized, ErrorBody{Success: false, Code: "unauthorized", Error: "unauthorized"})
}

func Forbidden(c *app.RequestContext, message string) {
	c.JSON(consts.StatusForbidden, ErrorBody{Success: false, Code: "forbidden", Error: "forbidden", Message: message})
}

func Internal(c *app.RequestContext, err error) {
	Error(c, consts.StatusInternalServerError, "internal_error", err)
}

func errMsg(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

func logError(c *app.RequestContext, err error) {
	if err == nil {
		return
	}
	slog.ErrorContext(context.Background(), "http error",
		"path", string(c.Path()),
		"method", string(c.Method()),
		"error", err,
	)
}
