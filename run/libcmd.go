package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	"github.com/replicatedcom/libcmd"
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
		"ContainerRepository": "emosbaugh/cmd",
	}
	libcmd.InitCmdContainer(opts)

	cmd := libcmd.NewCmd(op)
	log.Infof("Running command \"%s\"", op)
	result, err := cmd.Run(flag.Args()...)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Command result: %+v", result)
}
