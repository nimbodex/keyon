package network

import (
	"log/slog"
	"net"
	"sync"
	"time"
)

type Config struct {
	Address        string
	MaxConnections int
	MaxMessageSize int
	IdleTimeout    time.Duration
}

type Handler func(req string) string

type Server struct {
	address        string
	maxConnections int
	maxMessageSize int
	idleTimeout    time.Duration
	handler        Handler
	logger         *slog.Logger

	connSem  chan struct{}
	listener net.Listener
	wg       sync.WaitGroup
}

func NewServer(cfg Config, handler Handler, logger *slog.Logger) *Server {
	return &Server{
		address:        cfg.Address,
		maxConnections: cfg.MaxConnections,
		maxMessageSize: cfg.MaxMessageSize,
		idleTimeout:    cfg.IdleTimeout,
		handler:        handler,
		logger:         logger,
		connSem:        make(chan struct{}, cfg.MaxConnections),
	}
}
