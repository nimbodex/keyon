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

const initialScannerBuf = 4096

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
	connSem        chan struct{}
	wg             sync.WaitGroup

	mu       sync.Mutex
	listener net.Listener
	closed   bool
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

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		s.closeListener()
	}()

	defer s.wg.Wait()
	defer s.closeListener()

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			return acceptErr
		}

		select {
		case <-ctx.Done():
			_ = conn.Close()
			return nil
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

func (s *Server) closeListener() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.listener == nil {
		return
	}
	s.closed = true
	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.logger.Error("Server close", slog.Any("error", err))
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

func (s *Server) Stop() {
	s.closeListener()
	s.wg.Wait()
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
			err := scanner.Err()
			switch {
			case err == nil:
			case errors.Is(err, net.ErrClosed):
			case isTimeout(err):
			case errors.Is(err, bufio.ErrTooLong):
				_, _ = conn.Write([]byte("ERROR: Message too large\n"))
			default:
				s.logger.Error("Server scan", slog.Any("error", err))
			}
			return
		}

		res := s.handler(scanner.Text())
		if _, err := conn.Write([]byte(res + "\n")); err != nil {
			return
		}
	}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
