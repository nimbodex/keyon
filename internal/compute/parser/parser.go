package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nimbodex/keyon/internal/compute/command"
)

var (
	ErrEmptyInput     = errors.New("empty input")
	ErrUnknownCommand = errors.New("unknown command")
	ErrWrongNumArgs   = errors.New("wrong number of arguments")
)

var validArgRegex = regexp.MustCompile(`^\w+$`)

func Parse(input string) (command.Command, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return command.Command{}, ErrEmptyInput
	}

	args := tokens[1:]

	for _, arg := range args {
		if !validArgRegex.MatchString(arg) {
			return command.Command{}, fmt.Errorf("invalid argument: %s", arg)
		}
	}

	switch tokens[0] {
	case "SET":
		if len(args) != 2 {
			return command.Command{}, ErrWrongNumArgs
		}
		return command.Command{
			Type: command.CmdSet,
			Args: args,
		}, nil
	case "GET":
		if len(args) != 1 {
			return command.Command{}, ErrWrongNumArgs
		}
		return command.Command{
			Type: command.CmdGet,
			Args: args,
		}, nil
	case "DEL":
		if len(args) != 1 {
			return command.Command{}, ErrWrongNumArgs
		}
		return command.Command{
			Type: command.CmdDel,
			Args: args,
		}, nil
	default:
		return command.Command{}, ErrUnknownCommand
	}
}
