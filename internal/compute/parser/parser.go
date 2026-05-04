package parser

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/nimbodex/keyon/internal/compute/command"
)

var (
	ErrEmptyInput      = errors.New("empty input")
	ErrUnknownCommand  = errors.New("unknown command")
	ErrWrongNumArgs    = errors.New("wrong number of arguments")
	ErrInvalidArgument = errors.New("invalid argument")
)

const (
	setArgsCount = 2
	getArgsCount = 1
	delArgsCount = 1
)

func isValidArg(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func Parse(input string) (command.Command, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return command.Command{}, ErrEmptyInput
	}

	args := tokens[1:]

	var expectedArgs int
	var cmdType command.Type

	switch tokens[0] {
	case "SET":
		cmdType = command.CmdSet
		expectedArgs = setArgsCount
	case "GET":
		cmdType = command.CmdGet
		expectedArgs = getArgsCount
	case "DEL":
		cmdType = command.CmdDel
		expectedArgs = delArgsCount
	default:
		return command.Command{}, ErrUnknownCommand
	}

	if len(args) != expectedArgs {
		return command.Command{}, ErrWrongNumArgs
	}

	for _, arg := range args {
		if !isValidArg(arg) {
			return command.Command{}, fmt.Errorf("%w: %s", ErrInvalidArgument, arg)
		}
	}

	return command.Command{
		Type: cmdType,
		Args: args,
	}, nil
}
