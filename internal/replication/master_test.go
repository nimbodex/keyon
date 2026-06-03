package replication

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimbodex/keyon/pkg/logger"
)

func writeSegment(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
}

func TestMasterListEmptyDir(t *testing.T) {
	m := NewMaster(t.TempDir(), nil, logger.Nop())
	if got := m.Handle(cmdList); got != respOK {
		t.Fatalf("expected %q, got %q", respOK, got)
	}
}

func TestMasterListMissingDir(t *testing.T) {
	m := NewMaster(filepath.Join(t.TempDir(), "missing"), nil, logger.Nop())
	if got := m.Handle(cmdList); got != respOK {
		t.Fatalf("expected %q, got %q", respOK, got)
	}
}

func TestMasterListSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	writeSegment(t, dir, "wal_2.log", "SET b 2\n")
	writeSegment(t, dir, "wal_1.log", "SET a 1\n")
	writeSegment(t, dir, "wal_3.log", "SET c 3\n")
	writeSegment(t, dir, "notes.txt", "ignore me\n")

	m := NewMaster(dir, func() string { return "wal_3.log" }, logger.Nop())
	got := m.Handle(cmdList)

	want := respOK + " wal_1.log wal_2.log"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMasterGet(t *testing.T) {
	dir := t.TempDir()
	content := "SET a 1\nDEL a\n"
	writeSegment(t, dir, "wal_1.log", content)

	m := NewMaster(dir, nil, logger.Nop())
	got := m.Handle(cmdGet + " wal_1.log")

	payload, ok := strings.CutPrefix(got, respOK+" ")
	if !ok {
		t.Fatalf("expected OK response, got %q", got)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != content {
		t.Fatalf("expected %q, got %q", content, string(decoded))
	}
}

func TestMasterGetMissingSegment(t *testing.T) {
	m := NewMaster(t.TempDir(), nil, logger.Nop())
	if got := m.Handle(cmdGet + " wal_1.log"); !strings.HasPrefix(got, respError) {
		t.Fatalf("expected error response, got %q", got)
	}
}

func TestMasterRejectsInvalidNames(t *testing.T) {
	m := NewMaster(t.TempDir(), nil, logger.Nop())

	cases := []string{
		cmdGet + " ../etc/passwd",
		cmdGet + " wal_1.txt",
		cmdGet + " ../wal_1.log",
		cmdGet,
		"BOGUS",
		"",
	}
	for _, req := range cases {
		if got := m.Handle(req); !strings.HasPrefix(got, respError) {
			t.Fatalf("req %q: expected error, got %q", req, got)
		}
	}
}
