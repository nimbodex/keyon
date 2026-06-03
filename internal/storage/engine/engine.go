package engine

import (
	"errors"
	"hash/fnv"
	"log/slog"
	"sync"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

const defaultPartitions = 16

type partition struct {
	mu   sync.RWMutex
	data map[string]string
}

type Engine struct {
	partitions []*partition
	logger     *slog.Logger
}

func NewEngine(logger *slog.Logger) *Engine {
	return NewEngineWithPartitions(logger, defaultPartitions)
}

func NewEngineWithPartitions(logger *slog.Logger, count int) *Engine {
	if count <= 0 {
		count = defaultPartitions
	}
	partitions := make([]*partition, count)
	for i := range partitions {
		partitions[i] = &partition{data: make(map[string]string)}
	}
	return &Engine{
		partitions: partitions,
		logger:     logger,
	}
}

func (e *Engine) partition(key string) *partition {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return e.partitions[h.Sum32()%uint32(len(e.partitions))]
}

func (e *Engine) Set(key, val string) error {
	p := e.partition(key)
	p.mu.Lock()
	p.data[key] = val
	p.mu.Unlock()

	e.logger.Debug("set", slog.String("key", key), slog.String("val", val))

	return nil
}

func (e *Engine) Get(key string) (string, error) {
	p := e.partition(key)
	p.mu.RLock()
	val, ok := p.data[key]
	p.mu.RUnlock()

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
	p := e.partition(key)
	p.mu.Lock()
	delete(p.data, key)
	p.mu.Unlock()

	e.logger.Debug("del", slog.String("key", key))

	return nil
}
