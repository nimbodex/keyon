package wal

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nimbodex/keyon/internal/compute/command"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func readAllSegments(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), segmentPrefix) && strings.HasSuffix(e.Name(), segmentSuffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func readAllLines(t *testing.T, dir string) []string {
	t.Helper()
	var lines []string
	for _, name := range readAllSegments(t, dir) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		for l := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
			if l != "" {
				lines = append(lines, l)
			}
		}
	}
	return lines
}

func TestWALWriteSingle(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      10,
		BatchTimeout:   5 * time.Millisecond,
		MaxSegmentSize: 1024,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	if err := w.Write(Entry{Type: command.CmdSet, Args: []string{"k", "v"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	lines := readAllLines(t, dir)
	if len(lines) != 1 || lines[0] != "SET k v" {
		t.Fatalf("unexpected content: %v", lines)
	}
}

func TestWALWriteParallel(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      20,
		BatchTimeout:   5 * time.Millisecond,
		MaxSegmentSize: 1 << 20,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)
	for i := range N {
		go func(i int) {
			defer wg.Done()
			err := w.Write(Entry{
				Type: command.CmdSet,
				Args: []string{"k", "v"},
			})
			if err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	lines := readAllLines(t, dir)
	if len(lines) != N {
		t.Fatalf("expected %d lines, got %d", N, len(lines))
	}
	for _, l := range lines {
		if l != "SET k v" {
			t.Fatalf("unexpected line: %q", l)
		}
	}
}

func TestWALTimeoutFlush(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      1000,
		BatchTimeout:   10 * time.Millisecond,
		MaxSegmentSize: 1 << 20,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	defer w.Close()

	start := time.Now()
	if err := w.Write(Entry{Type: command.CmdSet, Args: []string{"a", "b"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("write took too long: %v", elapsed)
	}
	if elapsed < 5*time.Millisecond {
		t.Fatalf("flushed too eagerly: %v", elapsed)
	}
}

func TestWALSegmentRotation(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      1,
		BatchTimeout:   5 * time.Millisecond,
		MaxSegmentSize: 20,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	for i := range 5 {
		time.Sleep(1 * time.Millisecond)
		if err := w.Write(Entry{Type: command.CmdSet, Args: []string{"k", "v"}}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	segs := readAllSegments(t, dir)
	if len(segs) < 2 {
		t.Fatalf("expected at least 2 segments, got %d (%v)", len(segs), segs)
	}
	lines := readAllLines(t, dir)
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines total, got %d", len(lines))
	}
}

func TestWALCloseDrains(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      1000,
		BatchTimeout:   1 * time.Hour,
		MaxSegmentSize: 1 << 20,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	const N = 5
	var wg sync.WaitGroup
	wg.Add(N)
	for range N {
		go func() {
			defer wg.Done()
			_ = w.Write(Entry{Type: command.CmdSet, Args: []string{"k", "v"}})
		}()
	}

	time.Sleep(20 * time.Millisecond)

	closeDone := make(chan error, 1)
	go func() { closeDone <- w.Close() }()

	wg.Wait()
	if err := <-closeDone; err != nil {
		t.Fatalf("close: %v", err)
	}

	lines := readAllLines(t, dir)
	if len(lines) != N {
		t.Fatalf("expected %d lines after drain, got %d", N, len(lines))
	}
}

func TestWALWriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{DataDir: dir, BatchSize: 1, BatchTimeout: time.Millisecond, MaxSegmentSize: 1024}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	err = w.Write(Entry{Type: command.CmdSet, Args: []string{"k", "v"}})
	if !errors.Is(err, ErrWALClosed) {
		t.Fatalf("expected ErrWALClosed, got %v", err)
	}
}
