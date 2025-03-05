package handlers

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
)

var errHandler *ErrorHandler

// ErrorHandler - Handle custom events (logs and errors)
type ErrorHandler struct {
	app *pocketbase.PocketBase
}

// NewErrorHandler - ErrorHandler instance
func NewErrorHandler(app *pocketbase.PocketBase) *ErrorHandler {
	return &ErrorHandler{app: app}
}

// LogError - Custom errors
func (h *ErrorHandler) Error(status int, message string, errData interface{}, attrs ...interface{}) error {
	logAttrs := append([]interface{}{"status", status}, attrs...)
	if errData != nil {
		logAttrs = append(logAttrs, "data", errData)
	}

	switch status {
	case 400, 401, 403, 404:
		LogWarn(message, logAttrs...) // Usa a função LogWarn do mesmo pacote
	case 500:
		LogError(nil, message, logAttrs...) // Usa a função LogError do mesmo pacote
	default:
		LogError(nil, "Unknown error status", append(logAttrs, "originalMessage", message)...)
	}
	return apis.NewApiError(status, message, errData)
}

// InitErrorHandler - global component
func InitErrorHandler(app *pocketbase.PocketBase) {
	errHandler = NewErrorHandler(app)
	LogInfo("ErrorHandler initialized")
}

// BadRequestError - Validations
func BadRequestError(message string, errData interface{}, attrs ...interface{}) error {
	if errHandler == nil {
		return apis.NewApiError(500, "ErrorHandler not initialized", nil)
	}
	return errHandler.Error(400, message, errData, attrs...)
}

// UnauthorizedError - Invalid credentials
func UnauthorizedError(message string, errData interface{}, attrs ...interface{}) error {
	if errHandler == nil {
		return apis.NewApiError(500, "ErrorHandler not initialized", nil)
	}
	return errHandler.Error(401, message, errData, attrs...)
}

// ForbiddenError - User not authorized
func ForbiddenError(message string, errData interface{}, attrs ...interface{}) error {
	if errHandler == nil {
		return apis.NewApiError(500, "ErrorHandler not initialized", nil)
	}
	return errHandler.Error(403, message, errData, attrs...)
}

// NotFoundError - Resource not found
func NotFoundError(message string, errData interface{}, attrs ...interface{}) error {
	if errHandler == nil {
		return apis.NewApiError(500, "ErrorHandler not initialized", nil)
	}
	return errHandler.Error(404, message, errData, attrs...)
}

// InternalServerError - Internal errors (database, unknown exceptions)
func InternalServerError(message string, errData interface{}, attrs ...interface{}) error {
	if errHandler == nil {
		return apis.NewApiError(500, "ErrorHandler not initialized", nil)
	}
	return errHandler.Error(500, message, errData, attrs...)
}
