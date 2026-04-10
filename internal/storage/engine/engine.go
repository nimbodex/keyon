package engine

import (
	"errors"
	"log/slog"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

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

func (e *Engine) Set(key, val string) error {
	e.data[key] = val
	e.logger.Debug("set", slog.String("key", key), slog.String("val", val))

	return nil
}

func (e *Engine) Get(key string) (string, error) {
	val, ok := e.data[key]
	if !ok {
		e.logger.Error("get", slog.String("key", key), slog.String("val", val))
		return "", ErrKeyNotFound
	}
	e.logger.Debug("get", slog.String("key", key), slog.String("val", val))

	return val, nil
}

func (e *Engine) Del(key string) error {
	delete(e.data, key)
	e.logger.Debug("del", slog.String("key", key))

	return nil
}
