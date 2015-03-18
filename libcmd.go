package libcmd

import (
	"errors"
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

var (
	ErrCommandNotFound = errors.New("command not found")

	globalDockerClient *docker.Client
	config             cmdConfig

	cmdConfigDefaultOpts = map[string]string{
		"CommandsDir":         "/root/commands",
		"DockerEndpoint":      "unix:///var/run/docker.sock",
		"ContainerRepository": "freighterio/cmd",
		"ContainerTag":        "latest",
	}
)

type cmdConfig struct {
	CommandsDir         string
	DockerEndpoint      string
	ContainerRepository string
	ContainerTag        string
}

func InitCmdContainer(opts map[string]string) {
	config = cmdConfig{}
	for key, dflt := range cmdConfigDefaultOpts {
		field := reflect.ValueOf(&config).Elem().FieldByName(key)
		if value, ok := opts[key]; ok {
			field.SetString(value)
		} else {
			field.SetString(dflt)
		}
	}

	client, err := docker.NewClient(config.DockerEndpoint)
	if err != nil {
		log.Fatal(err)
	}
	globalDockerClient = client
	//if err := pullImage(globalDockerClient, config.ContainerRepository, config.ContainerTag); err != nil {
	//	log.Fatal(err)
	//}
}

func RunCommand(op string, args ...string) ([]string, error) {
	goCmd, err := newGoCommand(op)
	if err == nil {
		result, err := goCmd.Run(args...)
		return result, err
	}

	containerCmd, err := newContainerCmd(op)
	if err != nil {
		return nil, err
	}
	result, err := containerCmd.Run(args...)
	if err != nil {
		return nil, err
	}
	return []string{result}, nil
}
