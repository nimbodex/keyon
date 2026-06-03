package compute

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/nimbodex/keyon/internal/compute/command"
	"github.com/nimbodex/keyon/internal/compute/parser"
)

var ErrReadOnly = errors.New("read-only replica: writes are not allowed")

type Storage interface {
	Set(key, value string) error
	Get(key string) (string, error)
	Del(key string) error
}

type WAL interface {
	WriteCmd(typ command.Type, args []string) error
}

type Compute interface {
	Handle(input string) (string, error)
}

type compute struct {
	storage  Storage
	wal      WAL
	logger   *slog.Logger
	readOnly bool
}

func New(storage Storage, wal WAL, logger *slog.Logger) Compute {
	return &compute{storage: storage, wal: wal, logger: logger}
}

func NewReadOnly(storage Storage, logger *slog.Logger) Compute {
	return &compute{storage: storage, logger: logger, readOnly: true}
}

func (c *compute) Handle(input string) (string, error) {
	c.logger.Info("incoming request", slog.String("input", input))

	cmd, err := parser.Parse(input)
	if err != nil {
		c.logger.Error("parse error", slog.String("input", input), slog.String("error", err.Error()))
		return "", err
	}

	switch cmd.Type {
	case command.CmdSet:
		if c.readOnly {
			return "", ErrReadOnly
		}
		if c.wal != nil {
			if err := c.wal.WriteCmd(command.CmdSet, cmd.Args); err != nil {
				return "", err
			}
		}
		k, v := cmd.Args[0], cmd.Args[1]
		if err := c.storage.Set(k, v); err != nil {
			return "", err
		}
		return "OK", nil
	case command.CmdGet:
		val, err := c.storage.Get(cmd.Args[0])
		if err != nil {
			return "(nil)", err
		}
		return val, nil
	case command.CmdDel:
		if c.readOnly {
			return "", ErrReadOnly
		}
		if c.wal != nil {
			if err := c.wal.WriteCmd(command.CmdDel, cmd.Args); err != nil {
				return "", err
			}
		}
		if err := c.storage.Del(cmd.Args[0]); err != nil {
			return "", err
		}
		return "OK", nil
	default:
		return "", fmt.Errorf("invalid request: %s", input)
	}
}
