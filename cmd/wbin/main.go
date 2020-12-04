package main

import (
	"errors"
	"log"
	"os"

	"github.com/shupkg/wbin/cmd"
	"github.com/spf13/pflag"
)

func main() {
	pflag.ErrHelp = errors.New("")
	var p = cmd.WithFlag(pflag.CommandLine)
	pflag.Parse()
	p.Files = append(p.Files, pflag.Args()...)
	if err := p.Run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
