package main

import (
	"bytes"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

type Forwarder struct {
	conn *websocket.Conn
}

func RunForwarder(config *Config, lines chan CapturedLine, doneChan chan bool) {
	bufferedLines := make(chan CapturedLine, 100)
	sendDone := make(chan bool)
	go connectAndSendLoop(config, bufferedLines, sendDone)
	for {
		line, ok := <-lines
		if !ok { // program exit
			close(bufferedLines)
			select { // give sender some time to quit but don't wait forever
			case <-sendDone:
			case <-time.After(10 * time.Second):
			}
			break
		}
		select {
		case bufferedLines <- line:
		default:
		}
	}
	doneChan <- true
	close(doneChan)
}

func connectWebsocket(config *Config, lines chan CapturedLine) *websocket.Conn {
	url := config.GetSubmitLogUrl()
	for {
		conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			if err == websocket.ErrBadHandshake && resp != nil && resp.StatusCode/100 == 3 {
				location, err := resp.Location()
				if err != nil {
					Warn("Could not read Location header: %s", err)
				} else {
					url = location.String()
				}
			} else {
				Warn("Could not connect to remote websocket: %s", err)
				if resp != nil {
					Warn("  HTTP response status: %s", resp.Status)
				}
			}
		} else {
			return conn
		}
		for {
			select {
			case _, ok := <-lines: // drop lines
				if !ok { // program exit, don't reconnect
					return nil
				}
			case <-time.After(5 * time.Second): // reconnect delay
				break
			}
		}
	}
}

func connectAndSendLoop(config *Config, lines chan CapturedLine, sendDone chan bool) {
	for {
		conn := connectWebsocket(config, lines)
		if conn == nil { // program exit
			break
		}
		SayOut("Connected")
		var err error
		for line := range lines {
			err = conn.WriteMessage(websocket.TextMessage, encodeLine(&line))
			if err != nil {
				Warn("Could not write to websocket: %s", err)
				conn.Close()
				break
			}
		}
		if err == nil { // program exit
			break
		}
	}
	sendDone <- true
	close(sendDone)
}

func encodeLine(capturedLine *CapturedLine) []byte {
	var buffer bytes.Buffer
	if !capturedLine.Stderr {
		buffer.WriteString("O")
	} else {
		buffer.WriteString("E")
	}
	if !capturedLine.Eof {
		buffer.WriteString("L")
	} else {
		buffer.WriteString("C")
	}
	buffer.WriteString("|")
	tsMillis := capturedLine.Timestamp.UnixNano() / 1000000
	buffer.Write(strconv.AppendInt([]byte{}, tsMillis, 10))
	buffer.WriteString("|")
	buffer.Write(strconv.AppendUint([]byte{}, capturedLine.LineNumber, 10))
	buffer.WriteString("|")
	buffer.WriteString(capturedLine.Line)
	return buffer.Bytes()
}

func (fwd Forwarder) ForwardLines(lines chan CapturedLine) {
	var err error
	for capturedLine := range lines {
		err = fwd.conn.WriteMessage(websocket.TextMessage, encodeLine(&capturedLine))
		if err != nil {
			break
		}
	}
	fwd.conn.Close()
	if err != nil {
		Die("Could not send to remote websocket: %s", err)
	}
}
