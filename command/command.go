package command

import (
	"errors"
)

var (
	ErrCommandNotFound = errors.New("command not found")
	ErrCommandResponse = errors.New("error running command")
)

type CmdConfig struct {
	CommandsDir         string
	DockerEndpoint      string
	ContainerRepository string
	ContainerTag        string
}
