//go:build windows

package supervisor

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// listen opens the control channel for a server directory. Windows has no unix
// sockets to put in the folder, so the supervisor takes a loopback port and
// records it beside the log instead.
func listen(serverDir string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		ln.Close()
		return nil, fmt.Errorf("unexpected listener address %s", ln.Addr())
	}

	if err := os.WriteFile(portPath(serverDir), []byte(fmt.Sprint(addr.Port)), 0644); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// dial connects to the port the supervisor recorded.
func dial(serverDir string) (net.Conn, error) {
	data, err := os.ReadFile(portPath(serverDir))
	if err != nil {
		return nil, err
	}
	return net.Dial("tcp", "127.0.0.1:"+strings.TrimSpace(string(data)))
}

// cleanupSocket removes the port file once the supervisor is done with it.
func cleanupSocket(serverDir string) {
	_ = os.Remove(portPath(serverDir))
}
