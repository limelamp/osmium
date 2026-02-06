package shared

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

// This server can only accept inputs, cannot send outputs.
func StartBasicSocketServer(inputPipe io.WriteCloser) {

	// Main action
	l, err := net.Listen("tcp", "127.0.0.1:59072")
	if err != nil {
		fmt.Println("Error: Could not start listener. Is port 59072 in use?")
		return
	}
	defer l.Close() // Ensure listener is closed properly when program exits.

	for {
		conn, err := l.Accept() // Accept an incoming connection
		if err != nil {
			continue // If no incoming just repeat
		}

		// Handle the connection (in a goroutine so the server doesn't freeze)
		go func(c net.Conn) {
			defer c.Close()                // Once we are done, close the connection
			scanner := bufio.NewScanner(c) // Get the line from the client
			for scanner.Scan() {
				command := scanner.Text()
				// Write the command received from the socket into Java's Stdin
				fmt.Fprintln(inputPipe, command)
			}
		}(conn)
	}
}
