package libcmd

import (
	"encoding/base64"
	"math/rand"
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
