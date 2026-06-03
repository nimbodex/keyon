package wal

import (
	"errors"
	"fmt"

	"github.com/nimbodex/keyon/internal/compute/command"
	"github.com/nimbodex/keyon/internal/compute/parser"
)

var ErrInvalidEntry = errors.New("invalid wal entry")

type Entry struct {
	Type command.Type
	Args []string
}

func Encode(e Entry) []byte {
	switch e.Type {
	case command.CmdSet:
		return []byte("SET " + e.Args[0] + " " + e.Args[1] + "\n")
	case command.CmdDel:
		return []byte("DEL " + e.Args[0] + "\n")
	default:
		panic(fmt.Sprintf("wal: cannot encode entry of type %d", e.Type))
	}
}

func Decode(line []byte) (Entry, error) {
	cmd, err := parser.Parse(string(line))
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrInvalidEntry, err)
	}
	if cmd.Type == command.CmdGet {
		return Entry{}, fmt.Errorf("%w: GET is not allowed in wal", ErrInvalidEntry)
	}
	return Entry{Type: cmd.Type, Args: cmd.Args}, nil
}
