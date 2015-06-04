package command

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

var (
	availableCommands = []string{
		"cert",
		"random",
		"raw",
	}
)

type containerCmd struct {
	op           string
	config       CmdConfig
	dockerClient *docker.Client
}

func NewContainerCmd(op string, config CmdConfig, dockerClient *docker.Client) (*containerCmd, error) {
	exists := false
	for _, o := range availableCommands {
		if o == op {
			exists = true
			break
		}
	}
	if !exists {
		return nil, ErrCommandNotFound
	}
	cmd := containerCmd{op, config, dockerClient}
	return &cmd, nil
}

func (c *containerCmd) Run(args ...string) ([]string, error) {
	cmdParts := []string{"bash", fmt.Sprintf("%s/%s.sh", c.config.CommandsDir, c.op)}
	cmdParts = append(cmdParts, args...)
	container, err := createContainer(c.dockerClient, c.config.ContainerRepository, c.config.ContainerTag, cmdParts)
	if err != nil {
		return nil, err
	}
	defer removeContainer(c.dockerClient, container.ID)

	if err := startContainer(c.dockerClient, container.ID); err != nil {
		return nil, err
	}

	stopCh := make(chan bool)
	eventCh, err := getContainerEventCh(c.dockerClient, container.ID, stopCh)
	if err != nil {
		return nil, err
	}

	for {
		event := <-eventCh
		if event.Status == "die" {
			close(stopCh)
			break
		}
	}

	exitCode, err := getContainerExitCode(c.dockerClient, container.ID)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := getContainerLogs(c.config.DockerEndpoint, container.ID)
	if err != nil {
		return nil, err
	}

	if exitCode == 0 {
		return []string{strings.TrimSpace(stdout)}, nil
	}

	return []string{strings.TrimSpace(stderr)}, ErrCommandResponse
}

func PullImage(client *docker.Client, repository, tag string) error {
	reader, writer := io.Pipe()
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			log.Debugf(" -> %s", line)
		}
	}(reader)
	opts := docker.PullImageOptions{
		Repository:   repository,
		Tag:          tag,
		OutputStream: writer,
	}
	log.Debugf("pulling image %s:%s", repository, tag)
	if err := client.PullImage(opts, docker.AuthConfiguration{}); err != nil {
		return err
	}
	log.Debugf(" -> pulling image %s:%s complete", repository, tag)
	return nil
}

func createContainer(client *docker.Client, repository, tag string, cmdParts []string) (*docker.Container, error) {
	log.Debugf("creating container %s:%s", repository, tag)
	config := &docker.Config{
		Image: fmt.Sprintf("%s:%s", repository, tag),
		Cmd:   cmdParts,
	}
	opts := docker.CreateContainerOptions{
		Config: config,
	}
	container, err := client.CreateContainer(opts)
	if err != nil {
		log.Errorf(" -> error creating container %s:%s: %s", repository, tag, err)
		return nil, err
	}
	log.Debugf(" -> container %s:%s with id %s created", repository, tag, container.ID)
	return container, nil

}

func startContainer(client *docker.Client, containerID string) error {
	log.Debugf("starting container %s", containerID)
	hostConfig := &docker.HostConfig{}
	if err := client.StartContainer(containerID, hostConfig); err != nil {
		log.Errorf(" -> error starting container %s: %s", containerID, err)
		return err
	}
	log.Debugf(" -> container %s started", containerID)
	return nil
}

func removeContainer(client *docker.Client, containerID string) error {
	log.Debugf("removing container %s", containerID)
	opts := docker.RemoveContainerOptions{
		ID:            containerID,
		RemoveVolumes: false,
		Force:         true,
	}
	if err := client.RemoveContainer(opts); err != nil {
		log.Errorf(" -> error removing container %s: %s", containerID, err)
		return err
	}
	log.Debugf(" -> container %s removed", containerID)
	return nil
}

func getContainerEventCh(client *docker.Client, containerID string, stopCh chan bool) (<-chan *docker.APIEvents, error) {
	eventCh := make(chan *docker.APIEvents)

	listener := make(chan *docker.APIEvents)
	log.Debugf("adding container %s event listener", containerID)
	if err := client.AddEventListener(listener); err != nil {
		log.Errorf(" -> error adding container %s event listener: %s", containerID, err)
		return nil, err
	}
	log.Debugf(" -> container %s event listener added successfully", containerID)

	go func() {
		for {
			select {
			case event := <-listener:
				if event.ID == containerID {
					eventCh <- event
				}
				continue
			case <-stopCh:
				return
			}
		}
	}()

	return eventCh, nil
}

func getContainerExitCode(client *docker.Client, containerID string) (int, error) {
	log.Debugf("inspecting container %s", containerID)
	cntr, err := client.InspectContainer(containerID)
	if err != nil {
		log.Errorf(" -> error inspecting container %s: %s", containerID, err)
		return -1, err
	}
	log.Debugf(" -> container %s inspect success", containerID)
	return cntr.State.ExitCode, nil
}

func getContainerLogs(endpoint, containerID string) (string, string, error) {
	log.Debugf("getting container %s logs", containerID)
	stdout, stderr, _, err := makeRequest("GET", endpoint, fmt.Sprintf("/containers/%s/logs?follow=0&stderr=1&stdout=1", containerID))
	if err != nil {
		log.Errorf(" -> error making container %s logs request: %s", containerID, err)
		return "", "", err
	}
	log.Debugf(" -> container %s logs request complete", containerID)
	return string(stdout), string(stderr), nil
}

func makeRequest(method, endpoint, path string) ([]byte, []byte, int, error) {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return nil, nil, -1, err
	}
	req.Header.Set("User-Agent", "go-dockerclient")
	var resp *http.Response
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, -1, docker.ErrInvalidEndpoint
	}
	protocol := u.Scheme
	address := u.Path
	if protocol == "unix" {
		dial, err := net.Dial(protocol, address)
		if err != nil {
			return nil, nil, -1, err
		}
		defer dial.Close()
		clientconn := httputil.NewClientConn(dial, nil)
		resp, err = clientconn.Do(req)
		if err != nil {
			return nil, nil, -1, err
		}
		defer clientconn.Close()
	} else {
		resp, err = http.DefaultClient.Do(req)
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, nil, -1, docker.ErrConnectionRefused
		}
		return nil, nil, -1, err
	}
	var stdoutBuffer, stderrBuffer bytes.Buffer
	if _, err := stdCopy(&stdoutBuffer, &stderrBuffer, resp.Body); err != nil {
		return nil, nil, -1, err
	}
	bErr, _ := ioutil.ReadAll(&stderrBuffer)
	bOut, err := ioutil.ReadAll(&stdoutBuffer)
	return bOut, bErr, resp.StatusCode, err
}
