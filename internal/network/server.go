package network

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	initialScannerBuf     = 4096
	defaultMaxConnections = 100
	defaultMaxMessageSize = 4 * 1024
	defaultIdleTimeout    = 5 * time.Minute
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
	maxMessageSize int
	idleTimeout    time.Duration
	handler        Handler
	logger         *slog.Logger
	connSem        chan struct{}
	wg             sync.WaitGroup
}

func NewServer(cfg Config, handler Handler, logger *slog.Logger) *Server {
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = defaultMaxConnections
	}
	if cfg.MaxMessageSize <= 0 {
		cfg.MaxMessageSize = defaultMaxMessageSize
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	return &Server{
		address:        cfg.Address,
		maxMessageSize: cfg.MaxMessageSize,
		idleTimeout:    cfg.IdleTimeout,
		handler:        handler,
		logger:         logger,
		connSem:        make(chan struct{}, cfg.MaxConnections),
	}
}

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	defer s.wg.Wait()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		select {
		case s.connSem <- struct{}{}:
			s.wg.Go(func() {
				defer func() { <-s.connSem }()
				defer recoverPanic(s.logger, conn.RemoteAddr())
				s.handleConnection(ctx, conn)
			})
		default:
			_, _ = conn.Write([]byte("ERROR: Too many connections\n"))
			_ = conn.Close()
		}
	}
}

func recoverPanic(logger *slog.Logger, addr net.Addr) {
	if r := recover(); r != nil {
		logger.Error("panic in connection handler",
			slog.Any("panic", r),
			slog.String("addr", addr.String()),
		)
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.logger.Error("Server close", slog.Any("error", err))
		}
	}()

	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, min(initialScannerBuf, s.maxMessageSize)), s.maxMessageSize)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(s.idleTimeout)); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.logger.Error("Server set read deadline", slog.Any("error", err))
			}
			return
		}

		if !scanner.Scan() {
			s.handleScanErr(scanner.Err(), conn)
			return
		}

		res := s.handler(scanner.Text())
		if _, err := conn.Write([]byte(res + "\n")); err != nil {
			return
		}
	}
}

func (s *Server) handleScanErr(err error, conn net.Conn) {
	switch {
	case errors.Is(err, bufio.ErrTooLong):
		_, _ = conn.Write([]byte("ERROR: Message too large\n"))
	case err != nil && !errors.Is(err, net.ErrClosed) && !isTimeout(err):
		s.logger.Error("Server scan", slog.Any("error", err))
	}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
