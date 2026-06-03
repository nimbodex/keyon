package wal

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nimbodex/keyon/internal/compute/command"
)

type fakeApplier struct {
	mu   sync.Mutex
	data map[string]string
}

func newFakeApplier() *fakeApplier {
	return &fakeApplier{data: make(map[string]string)}
}

func (f *fakeApplier) Set(k, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[k] = v
	return nil
}

func (f *fakeApplier) Get(k string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[k]
	return v, ok
}

func (f *fakeApplier) Del(k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, k)
	return nil
}

func TestRecoverMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("expected nil for missing dir, got %v", err)
	}
	if len(applier.data) != 0 {
		t.Fatalf("expected empty applier, got %v", applier.data)
	}
}

func TestRecoverEmptyDir(t *testing.T) {
	dir := t.TempDir()
	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applier.data) != 0 {
		t.Fatalf("expected empty applier")
	}
}

func writeSegment(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
}

func TestRecoverSingleSegment(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_1.log", "SET k v\nSET k v2\n")

	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	v, ok := applier.Get("k")
	if !ok || v != "v2" {
		t.Fatalf("expected k=v2, got %q (ok=%v)", v, ok)
	}
}

func TestRecoverMultipleSegmentsOrdering(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_1.log", "SET k v\n")
	writeSegment(t, dir, "wal_2.log", "DEL k\n")

	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, ok := applier.Get("k"); ok {
		t.Fatal("expected k to be deleted")
	}
}

func TestRecoverSkipsBrokenLine(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_1.log", "SET a 1\nBROKEN entry !!!\nSET b 2\n")

	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if v, ok := applier.Get("a"); !ok || v != "1" {
		t.Fatalf("expected a=1, got %q", v)
	}
	if v, ok := applier.Get("b"); !ok || v != "2" {
		t.Fatalf("expected b=2, got %q", v)
	}
}

func TestRecoverIgnoresNonWALFiles(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_1.log", "SET a 1\n")
	writeSegment(t, dir, "notes.txt", "should be ignored\n")
	writeSegment(t, dir, "wal_bad.dat", "SET nope 1\n")

	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, ok := applier.Get("nope"); ok {
		t.Fatal("expected non-WAL file to be ignored")
	}
	if v, _ := applier.Get("a"); v != "1" {
		t.Fatalf("expected a=1, got %q", v)
	}
}

type failingApplier struct{}

func (failingApplier) Set(string, string) error { return errors.New("fail") }
func (failingApplier) Del(string) error         { return errors.New("fail") }

func TestRecoverContinuesOnApplyError(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_1.log", "SET a 1\nSET b 2\n")
	if err := Recover(dir, failingApplier{}, testLogger()); err != nil {
		t.Fatalf("recover should not fail: %v", err)
	}
}

func TestRecoverE2E(t *testing.T) {
	dir := t.TempDir()
	w, err := New(Config{
		DataDir:        dir,
		BatchSize:      5,
		BatchTimeout:   5 * time.Millisecond,
		MaxSegmentSize: 1 << 20,
	}, testLogger())
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	ops := []Entry{
		{Type: command.CmdSet, Args: []string{"a", "1"}},
		{Type: command.CmdSet, Args: []string{"b", "2"}},
		{Type: command.CmdSet, Args: []string{"a", "11"}},
		{Type: command.CmdDel, Args: []string{"b"}},
	}
	for _, e := range ops {
		if err := w.Write(e); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	applier := newFakeApplier()
	if err := Recover(dir, applier, testLogger()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if v, ok := applier.Get("a"); !ok || v != "11" {
		t.Fatalf("expected a=11, got %q", v)
	}
	if _, ok := applier.Get("b"); ok {
		t.Fatal("expected b to be absent")
	}
}
