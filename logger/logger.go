package logger

import (
	"encoding/json"
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger

func init() {
	defaultLogger = slog.New(&FancyTextHandler{
		writer: os.Stdout,
		level:  slog.LevelDebug,
	})

}

func PrintObj(obj any) string {
	bytes, _ := json.MarshalIndent(obj, "\t", "\t")
	return string(bytes)
}

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

func Fatal(err error, args ...any) {
	defaultLogger.Error(err.Error(), args...)
	os.Exit(1)
}
