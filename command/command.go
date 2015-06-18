package command

import (
	"errors"
)

var (
	ErrCommandNotFound = errors.New("command not found")
	ErrMissingArgs     = errors.New("Missing required arguments")
)

type ErrCommandResponse struct {
	msg string
}

func (e ErrCommandResponse) Error() string {
	return e.msg
}

type CmdConfig struct {
	CommandsDir         string
	DockerEndpoint      string
	ContainerRepository string
	ContainerTag        string
}
