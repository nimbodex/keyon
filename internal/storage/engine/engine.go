package engine

import "log/slog"

type Engine struct {
	data   map[string]string
	logger *slog.Logger
}

func NewEngine(logger *slog.Logger) *Engine {
	return &Engine{
		data:   make(map[string]string),
		logger: logger,
	}
}
