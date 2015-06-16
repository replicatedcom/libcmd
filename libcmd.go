package libcmd

import (
	"net/http"
	"net/url"
	"reflect"

	"github.com/replicatedcom/libcmd/command"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

var (
	globalDockerClient *docker.Client
	config             command.CmdConfig

	cmdConfigDefaultOpts = map[string]string{
		"CommandsDir":         "/root/commands",
		"DockerEndpoint":      "unix:///var/run/docker.sock",
		"ContainerRepository": "freighterio/cmd",
		"ContainerTag":        "latest",
		"HttpProxy":           "",
	}
)

func InitCmdContainer(opts map[string]string) {
	config = command.CmdConfig{}
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

	if config.HttpProxy != "" {
		p := config.HttpProxy
		client.HTTPClient.Transport = &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse(p)
			},
		}
	}

	globalDockerClient = client
	if err := command.PullImage(globalDockerClient, config.ContainerRepository, config.ContainerTag); err != nil {
		log.Fatal(err)
	}
}

func RunCommand(op string, args ...string) ([]string, error) {
	goCmd, err := command.NewGoCmd(op, config, globalDockerClient)
	if err == nil {
		return goCmd.Run(args...)
	}
	if err != command.ErrCommandNotFound {
		return nil, err
	}

	containerCmd, err := command.NewContainerCmd(op, config, globalDockerClient)
	if err == nil {
		return containerCmd.Run(args...)
	}
	return nil, err
}
