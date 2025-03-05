package handlers

import (
	"log/slog"
	"os"

	"github.com/pocketbase/pocketbase"
)

var log *slog.Logger

func InitLogger(app *pocketbase.PocketBase) {
	log = app.Logger()

	consoleHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	log = slog.New(consoleHandler)

	log.Info("Custom log initialized")
}

// LogInfo message
func LogInfo(msg string, attrs ...interface{}) {
	log.Info(msg, attrs...)
}

// LogWarn message
func LogWarn(msg string, attrs ...interface{}) {
	log.Warn(msg, attrs...)
}

// LogError message
func LogError(err error, msg string, attrs ...interface{}) {
	attrs = append(attrs, "error", err)
	log.Error(msg, attrs...)
}

// LogDebug message
func LogDebug(msg string, attrs ...interface{}) {
	log.Debug(msg, attrs...)
}

// LogWith attributes message
func LogWith(attrs ...interface{}) *slog.Logger {
	return log.With(attrs...)
}

// LogWithGroup by name message
func LogWithGroup(name string) *slog.Logger {
	return log.WithGroup(name)
}
