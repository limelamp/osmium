//go:build !windows

package supervisor

import (
	"net"
	"os"
	"path/filepath"
)

// listen opens the control socket for a server directory. A unix socket lives
// in the server's own folder, so two servers can never collide on it and its
// permissions are the folder's.
func listen(serverDir string) (net.Listener, error) {
	path := socketPath(serverDir)

	// The fallback location outside the server folder has to be created; the
	// in-folder one is made along with the log.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	// A socket left behind by a supervisor that was killed would refuse to
	// bind; nothing is listening on it, so it is safe to clear.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return net.Listen("unix", path)
}

// dial connects to a server's control socket.
func dial(serverDir string) (net.Conn, error) {
	return net.Dial("unix", socketPath(serverDir))
}

// cleanupSocket removes the socket once the supervisor is done with it.
func cleanupSocket(serverDir string) {
	_ = os.Remove(socketPath(serverDir))
}
