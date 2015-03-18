package libcmd

import (
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
		results[i] = strings.TrimSpace(result)
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
