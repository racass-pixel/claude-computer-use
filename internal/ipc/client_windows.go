//go:build windows

package ipc

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

type Reply struct {
	Pipe string
	Body json.RawMessage
	Err  error
}

func listServers() []string {
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), Prefix) {
			out = append(out, `\\.\pipe\`+e.Name())
		}
	}
	return out
}

// Broadcast sends cmd to every running cu server and collects replies.
func Broadcast(cmd string, timeout time.Duration) []Reply {
	var replies []Reply
	for _, name := range listServers() {
		r := Reply{Pipe: name}
		conn, err := winio.DialPipe(name, &timeout)
		if err != nil {
			r.Err = err
			replies = append(replies, r)
			continue
		}
		conn.SetDeadline(time.Now().Add(timeout))
		if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
			r.Err = err
		} else if line, err := bufio.NewReader(conn).ReadString('\n'); err != nil && line == "" {
			r.Err = err
		} else {
			r.Body = json.RawMessage(strings.TrimSpace(line))
		}
		conn.Close()
		replies = append(replies, r)
	}
	return replies
}
