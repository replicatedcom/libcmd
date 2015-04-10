package libcmd

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

var (
	randCharset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-0123456789")
)

type goCommand interface {
	Run(args ...string) ([]string, error)
}

func newGoCommand(op string) (goCommand, error) {
	if op == "cert" {
		return &goCertCommand{}, nil
	} else if op == "random" {
		return &goRandCommand{}, nil
	} else if op == "echo" {
		return &goEchoCommand{}, nil
	} else if op == "publicip" {
		return &goPublicIpCommand{}, nil
	}

	return nil, ErrCommandNotFound
}

type goCertCommand struct {
}

func (c *goCertCommand) Run(args ...string) ([]string, error) {
	cmd, err := newContainerCmd("cert")
	if err != nil {
		return nil, err
	}
	result, err := cmd.Run(args...)
	if err != nil {
		return nil, err
	}
	results := strings.SplitAfter(result, "-----END RSA PRIVATE KEY-----")
	for i, result := range results {
		result := strings.TrimSpace(result)
		results[i] = base64.StdEncoding.EncodeToString([]byte(result))
	}
	return results, nil
}

type goRandCommand struct {
}

func (c *goRandCommand) Run(args ...string) ([]string, error) {
	length := 16
	if len(args) > 0 {
		var err error
		length, err = strconv.Atoi(args[0])
		if err != nil {
			return nil, err
		}
	}
	str := randSeq(length)
	return []string{str}, nil
}

func randSeq(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = randCharset[rand.Intn(len(randCharset))]
	}
	return string(b)
}

type goEchoCommand struct {
}

func (c *goEchoCommand) Run(args ...string) ([]string, error) {
	result := strings.Join(args, " ")
	return []string{result}, nil
}

type goPublicIpCommand struct {
	done   chan string
	errors chan error
}

func (c *goPublicIpCommand) Run(args ...string) ([]string, error) {
	urls := []string{
		"http://ipecho.net/plain",
		"http://ip.appspot.com",
		"http://whatismyip.akamai.com",
	}

	c.done = make(chan string)
	c.errors = make(chan error, len(urls))
	for _, url := range urls {
		go func(url string) {
			resp, err := http.Get(url)
			if err != nil {
				c.errors <- err
				return
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				c.errors <- err
				return
			}
			c.done <- string(body)
		}(url)
	}

	errCount := 0
	for {
		select {
		case result := <-c.done:
			return []string{result}, nil
		case <-c.errors:
			errCount = errCount + 1
			if errCount == len(urls) {
				return nil, errors.New("Error contacting publicip servers")
			}
		}
	}
}
