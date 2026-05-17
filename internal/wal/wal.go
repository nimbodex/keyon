package wal

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/nimbodex/keyon/internal/compute/command"
)

var ErrWALClosed = errors.New("wal closed")

type Config struct {
	DataDir        string
	BatchSize      int
	BatchTimeout   time.Duration
	MaxSegmentSize int64
}

type request struct {
	data []byte
	done chan error
}

type WAL struct {
	cfg      Config
	seg      *Segment
	reqs     chan request
	stop     chan struct{}
	doneLoop chan struct{}
	logger   *slog.Logger

	mu     sync.Mutex
	closed bool
	failed error
}

func New(cfg Config, logger *slog.Logger) (*WAL, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = 10 * time.Millisecond
	}
	if cfg.MaxSegmentSize <= 0 {
		cfg.MaxSegmentSize = 10 * 1024 * 1024
	}
	seg, err := openSegment(cfg.DataDir, cfg.MaxSegmentSize)
	if err != nil {
		return nil, err
	}
	w := &WAL{
		cfg:      cfg,
		seg:      seg,
		reqs:     make(chan request, cfg.BatchSize),
		stop:     make(chan struct{}),
		doneLoop: make(chan struct{}),
		logger:   logger,
	}
	go w.loop()
	return w, nil
}

func (w *WAL) WriteCmd(typ command.Type, args []string) error {
	return w.Write(Entry{Type: typ, Args: args})
}

func (w *WAL) Write(e Entry) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrWALClosed
	}
	if w.failed != nil {
		err := w.failed
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()

	done := make(chan error, 1)
	select {
	case w.reqs <- request{data: Encode(e), done: done}:
	case <-w.stop:
		return ErrWALClosed
	}
	return <-done
}

func (w *WAL) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.stop)
	<-w.doneLoop
	if w.seg != nil {
		return w.seg.Close()
	}
	return nil
}

func (w *WAL) loop() {
	defer close(w.doneLoop)

	batch := make([]request, 0, w.cfg.BatchSize)
	timer := time.NewTimer(w.cfg.BatchTimeout)
	if !timer.Stop() {
		<-timer.C
	}
	timerActive := false

	resetTimer := func() {
		if timerActive {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timerActive = false
		}
	}

	for {
		select {
		case req := <-w.reqs:
			if len(batch) == 0 {
				timer.Reset(w.cfg.BatchTimeout)
				timerActive = true
			}
			batch = append(batch, req)
			if len(batch) >= w.cfg.BatchSize {
				w.flush(batch)
				batch = batch[:0]
				resetTimer()
			}
		case <-timer.C:
			timerActive = false
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-w.stop:
			resetTimer()
		drain:
			for {
				select {
				case req := <-w.reqs:
					batch = append(batch, req)
				default:
					break drain
				}
			}
			if len(batch) > 0 {
				w.flush(batch)
			}
			return
		}
	}
}

func (w *WAL) flush(batch []request) {
	w.mu.Lock()
	if w.failed != nil {
		err := w.failed
		w.mu.Unlock()
		for _, r := range batch {
			r.done <- err
		}
		return
	}
	w.mu.Unlock()

	total := 0
	for _, r := range batch {
		total += len(r.data)
	}

	if w.seg.Full(total) {
		if err := w.seg.Close(); err != nil {
			w.fail(batch, err)
			return
		}
		seg, err := openSegment(w.cfg.DataDir, w.cfg.MaxSegmentSize)
		if err != nil {
			w.fail(batch, err)
			return
		}
		w.seg = seg
	}

	buf := make([]byte, 0, total)
	for _, r := range batch {
		buf = append(buf, r.data...)
	}

	if err := w.seg.Append(buf); err != nil {
		w.fail(batch, err)
		return
	}
	if err := w.seg.Sync(); err != nil {
		w.fail(batch, err)
		return
	}

	for _, r := range batch {
		r.done <- nil
	}
}

func (w *WAL) fail(batch []request, err error) {
	w.logger.Error("wal flush failed", slog.Any("error", err))
	w.mu.Lock()
	if w.failed == nil {
		w.failed = err
	}
	w.mu.Unlock()
	for _, r := range batch {
		r.done <- err
	}
}
