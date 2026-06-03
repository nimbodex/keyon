package wal

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const segmentPrefix = "wal_"
const segmentSuffix = ".log"

var ErrSegmentClosed = errors.New("segment closed")

type Segment struct {
	file    *os.File
	path    string
	size    int64
	maxSize int64
	closed  bool
}

func openSegment(dir string, maxSize int64) (*Segment, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := segmentPrefix + strconv.FormatInt(time.Now().UnixNano(), 10) + segmentSuffix
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Segment{file: f, path: path, maxSize: maxSize}, nil
}

func (s *Segment) Append(data []byte) error {
	if s.closed {
		return ErrSegmentClosed
	}
	n, err := s.file.Write(data)
	s.size += int64(n)
	return err
}

func (s *Segment) Sync() error {
	if s.closed {
		return ErrSegmentClosed
	}
	return s.file.Sync()
}

func (s *Segment) Full(extra int) bool {
	return s.size+int64(extra) > s.maxSize
}

func (s *Segment) Size() int64 {
	return s.size
}

func (s *Segment) Path() string {
	return s.path
}

func (s *Segment) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.file.Close()
}
