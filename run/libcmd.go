package main

import (
	"flag"

	"github.com/replicatedcom/libcmd"
	"github.com/replicatedcom/libcmd/command"

	log "github.com/Sirupsen/logrus"
)

var (
	op string
)

func init() {
	flag.StringVar(&op, "cmd", "random", "command to run")
	flag.Parse()
}

func main() {
	log.SetLevel(log.DebugLevel)

	opts := map[string]string{
		"ContainerRepository": "freighter/cmd",
		"ContainerTag":        "latest",
	}
	libcmd.InitCmdContainer(opts)

	log.Infof("Running command \"%s\"", op)

	results, err := libcmd.RunCommand(op, flag.Args()...)

	if _, ok := err.(command.ErrCommandResponse); ok {
		log.Errorf("Command error:\nError: %v\nResults: %q", err, results)
	} else if err == nil {
		log.Infof("Command success\nResults: %q", results)
	} else {
		log.Fatal(err)
	}
}
