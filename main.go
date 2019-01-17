package main

import (
	"os"
)

const DevVersion = "git"

var Version string = DevVersion

func main() {
	SayOut("Version: %s", Version)
	if len(os.Args) < 2 {
		Die("Usage: rdoctor <command>")
	}
	config := LoadConfig()
	CheckForUpdate(config)
	if !config.HasApiKey() {
		RunSetup(config)
	}
	exitChan := make(chan int)
	doneChan := make(chan bool)
	lines := make(chan CapturedLine)
	go RunForwarder(config, lines, doneChan)
	StartMainProgram(os.Args[1:], lines, exitChan)
	exitCode := <-exitChan
	<-doneChan
	os.Exit(exitCode)
}
