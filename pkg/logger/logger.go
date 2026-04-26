package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

func Nop() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func NewLogger(level, output string) (*slog.Logger, io.Closer, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, nil, err
	}

	w, closer, err := openWriter(output)
	if err != nil {
		return nil, nil, err
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})), closer, nil
}

func parseLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", level)
	}
}

func openWriter(output string) (io.Writer, io.Closer, error) {
	switch output {
	case "stdout":
		return os.Stdout, nopCloser{}, nil
	case "stderr":
		return os.Stderr, nopCloser{}, nil
	default:
		file, err := os.OpenFile(
			output,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644,
		)
		if err != nil {
			return nil, nil, err
		}

		return file, file, nil
	}
}

type nopCloser struct{}

func (nopCloser) Close() error {
	return nil
}
