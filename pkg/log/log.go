package log

import (
	"io"
	"log/slog"
	"sync"
)

func Setup(out io.Writer, level slog.Level) {
	sync.OnceFunc(func() {
		handler := slog.NewJSONHandler(out, &slog.HandlerOptions{AddSource: true, Level: level})
		logger := slog.New(handler)
		slog.SetDefault(logger)
	})()
}
