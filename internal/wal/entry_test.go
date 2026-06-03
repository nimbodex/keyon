package wal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/nimbodex/keyon/internal/compute/command"
)

func TestEncodeSet(t *testing.T) {
	got := Encode(Entry{Type: command.CmdSet, Args: []string{"k", "v"}})
	want := []byte("SET k v\n")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEncodeDel(t *testing.T) {
	got := Encode(Entry{Type: command.CmdDel, Args: []string{"k"}})
	want := []byte("DEL k\n")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEncodeGetPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	Encode(Entry{Type: command.CmdGet, Args: []string{"k"}})
}

func TestDecodeSet(t *testing.T) {
	e, err := Decode([]byte("SET k v"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Type != command.CmdSet || !reflect.DeepEqual(e.Args, []string{"k", "v"}) {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestDecodeDel(t *testing.T) {
	e, err := Decode([]byte("DEL k"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Type != command.CmdDel || !reflect.DeepEqual(e.Args, []string{"k"}) {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestDecodeGetRejected(t *testing.T) {
	_, err := Decode([]byte("GET k"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("expected ErrInvalidEntry, got %v", err)
	}
}

func TestDecodeGarbage(t *testing.T) {
	_, err := Decode([]byte("nonsense !!!"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("expected ErrInvalidEntry, got %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	entries := []Entry{
		{Type: command.CmdSet, Args: []string{"foo", "bar"}},
		{Type: command.CmdSet, Args: []string{"a", "b"}},
		{Type: command.CmdDel, Args: []string{"x"}},
	}
	for _, e := range entries {
		enc := Encode(e)
		dec, err := Decode(enc[:len(enc)-1])
		if err != nil {
			t.Fatalf("decode(%q): %v", enc, err)
		}
		if dec.Type != e.Type || !reflect.DeepEqual(dec.Args, e.Args) {
			t.Fatalf("round-trip failed: in=%+v out=%+v", e, dec)
		}
	}
}
