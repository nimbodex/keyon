package wal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenSegmentCreatesDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nested", "wal")
	seg, err := openSegment(dir, 1024)
	if err != nil {
		t.Fatalf("openSegment: %v", err)
	}
	defer seg.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory")
	}
	if !strings.HasPrefix(filepath.Base(seg.Path()), segmentPrefix) {
		t.Fatalf("expected wal_ prefix, got %s", seg.Path())
	}
}

func TestSegmentAppendAndSync(t *testing.T) {
	dir := t.TempDir()
	seg, err := openSegment(dir, 1024)
	if err != nil {
		t.Fatalf("openSegment: %v", err)
	}
	defer seg.Close()

	if err := seg.Append([]byte("hello\n")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if seg.Size() != int64(len("hello\n")) {
		t.Fatalf("expected size %d, got %d", len("hello\n"), seg.Size())
	}
	if err := seg.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	data, err := os.ReadFile(seg.Path())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("expected %q, got %q", "hello\n", string(data))
	}
}

func TestSegmentFull(t *testing.T) {
	dir := t.TempDir()
	seg, err := openSegment(dir, 10)
	if err != nil {
		t.Fatalf("openSegment: %v", err)
	}
	defer seg.Close()

	if seg.Full(5) {
		t.Fatal("expected not full")
	}
	if err := seg.Append([]byte("aaaaaa")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if seg.Full(4) {
		t.Fatalf("expected not full at boundary (size=%d + extra=4, max=10)", seg.Size())
	}
	if !seg.Full(5) {
		t.Fatalf("expected full at size=%d + extra=5 > 10", seg.Size())
	}
}

func TestSegmentClose(t *testing.T) {
	dir := t.TempDir()
	seg, err := openSegment(dir, 1024)
	if err != nil {
		t.Fatalf("openSegment: %v", err)
	}
	if err := seg.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := seg.Append([]byte("x")); !errors.Is(err, ErrSegmentClosed) {
		t.Fatalf("expected ErrSegmentClosed, got %v", err)
	}
	if err := seg.Sync(); !errors.Is(err, ErrSegmentClosed) {
		t.Fatalf("expected ErrSegmentClosed on sync, got %v", err)
	}
	if err := seg.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}
