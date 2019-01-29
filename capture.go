package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type CapturedLine struct {
	Timestamp  time.Time
	Line       string
	LineNumber uint64
	Stderr     bool
	Eof        bool
}

func (c CapturedLine) String() string {
	origin := "STDOUT"
	if c.Stderr {
		origin = "STDERR"
	}
	format := "%s %v L%03d %s"
	if c.Eof {
		format = "%s %v EOF L%03d %s"
	}
	return fmt.Sprintf(format, origin, c.Timestamp, c.LineNumber, c.Line)
}

func readLines(pipe io.ReadCloser, lines chan CapturedLine, stderr bool) {
	defer close(lines)
	copyOut := os.Stdout
	if stderr {
		copyOut = os.Stderr
	}
	scanner := bufio.NewScanner(pipe)
	var lineNumber uint64 = 1
	for {
		eof := !scanner.Scan()
		capturedLine := CapturedLine{
			Timestamp:  time.Now(),
			Line:       scanner.Text(),
			LineNumber: lineNumber,
			Stderr:     stderr,
			Eof:        eof,
		}
		select {
		case lines <- capturedLine:
		default:
		}
		if eof {
			break
		}
		fmt.Fprintln(copyOut, capturedLine.Line)
		lineNumber++
	}
	if err := scanner.Err(); err != nil {
		SayErr("Could not read from pipe: %s", err)
	}
	// let cmd.Wait() close pipe
}

func combineOutputs(stdout, stderr io.ReadCloser, lines chan CapturedLine, eofChan chan bool) {
	stdoutLines := make(chan CapturedLine)
	stderrLines := make(chan CapturedLine)
	go readLines(stdout, stdoutLines, false)
	go readLines(stderr, stderrLines, true)
	for {
		select {
		case line, ok := <-stdoutLines:
			if ok {
				lines <- line
			} else {
				stdoutLines = nil
			}
		case line, ok := <-stderrLines:
			if ok {
				lines <- line
			} else {
				stderrLines = nil
			}
		}
		if stdoutLines == nil && stderrLines == nil {
			break
		}
	}
	close(lines)
	eofChan <- true
	close(eofChan)
}

func StartMainProgram(cmdLine []string, lines chan CapturedLine, exitChan chan int) {
	var err error
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	var stdout, stderr io.ReadCloser
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		Die("Could not create pipe: %s", err)
	}
	stderr, err = cmd.StderrPipe()
	if err != nil {
		Die("Could not create pipe: %s", err)
	}
	/*
	 * cmd.Wait() closes output pipes on process exit which makes
	 * bufio.Scanner's read() fail. Wait for EOF (from both stdout and stdin)
	 * before calling cmd.Wait()
	 */
	eofChan := make(chan bool)
	go combineOutputs(stdout, stderr, lines, eofChan)
	interrupts := make(chan os.Signal)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			sig := <-interrupts
			// cmd.Process is nil before cmd.Start() succeeds
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()
	err = cmd.Start()
	if err != nil {
		Die("Could not create process: %s", err)
	}
	go func() {
		exitCode := 0
		<-eofChan
		err := cmd.Wait()
		if err != nil {
			// https://stackoverflow.com/a/10385867/214720
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
					err = nil
				}
			}
		}
		if err != nil {
			Warn("Waiting for command was not successful: %s", err)
		}
		exitChan <- exitCode
		close(exitChan)
	}()
}
