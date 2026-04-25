package engine

import (
	"errors"
	"log/slog"
	"sync"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type Engine struct {
	mu     sync.RWMutex
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
	e.mu.Lock()
	e.data[key] = val
	e.mu.Unlock()

	e.logger.Debug("set", slog.String("key", key), slog.String("val", val))

	return nil
}

func (e *Engine) Get(key string) (string, error) {
	e.mu.RLock()
	val, ok := e.data[key]
	e.mu.RUnlock()

	e.logger.Debug("get",
		slog.String("key", key),
		slog.String("val", val),
		slog.Bool("found", ok),
	)

	if !ok {
		return "", ErrKeyNotFound
	}
	return val, nil
}

func (e *Engine) Del(key string) error {
	e.mu.Lock()
	delete(e.data, key)
	e.mu.Unlock()

	e.logger.Debug("del", slog.String("key", key))

	return nil
}
