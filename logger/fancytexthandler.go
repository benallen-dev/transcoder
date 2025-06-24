package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type FancyTextHandler struct {
	writer io.Writer
	level  slog.Level
}

func (h *FancyTextHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level
}

func renderLevel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "[\x1b[0;37m DEBUG \x1b[0m]"
	case slog.LevelInfo:
		return "[\x1b[0;32m INFO \x1b[0m]"
	case slog.LevelWarn:
		return "[\x1b[33m WARN \x1b[0m]"
	case slog.LevelError:
		return "[\x1b[31m ERROR \x1b[0m]"
	default:
		return "[\x1b[32m ??? \x1b[0m]"
	}
}

func (h *FancyTextHandler) Handle(_ context.Context, r slog.Record) error {

	out := ""

	out += fmt.Sprintf("\x1b[34m%s\x1b[0m %s %s\n",
		r.Time.Format("15:04:05"),
		renderLevel(r.Level),
		r.Message,
	)

	if r.NumAttrs() > 0 {
		attrs := []string{}

		r.Attrs(func(attr slog.Attr) bool {
		 	attrs = append(attrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value))
			return true
		})

		out += strings.Join(attrs, ", ")
		out += "\n"
	}

	fmt.Fprint(h.writer, out)
	return nil
}

func (h *FancyTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Implement if you care about structured fields
	return h
}

func (h *FancyTextHandler) WithGroup(name string) slog.Handler {
	return h // Grouping is no-op for now
}
