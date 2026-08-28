// Package supervisor runs a Minecraft server as a process of its own, outside
// whichever Osmium invocation started it.
//
// A server used to be a child of the TUI, which meant it died when the TUI did
// and could only be typed at from the session that launched it. Here a detached
// supervisor process owns the server instead: it holds the server's stdin, tees
// its output into a log file, and listens on a control socket in the server's
// own folder. Anything that wants to talk to that server — a later TUI, the
// `osmium exec` command — connects to the socket and tails the log, so every
// Osmium invocation sees the same server the same way.
package supervisor

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/limelamp/osmium/internal/shared"
)

// The per-server state folder and the files the supervisor keeps in it.
const (
	StateDirName = ".osmium"
	logFileName  = "server.log"
	sockFileName = "control.sock"
	portFileName = "control.port"
)

// ErrNotRunning is returned when a request is made of a server that has no
// supervisor to answer it.
var ErrNotRunning = errors.New("server is not running")

// ErrAlreadyRunning is returned when a server is asked to start twice.
var ErrAlreadyRunning = errors.New("server is already running")

// StateDir is where a server directory keeps its supervisor's files.
func StateDir(serverDir string) string {
	if serverDir == "" {
		serverDir = "."
	}
	return filepath.Join(serverDir, StateDirName)
}

// LogPath is the combined output log of a server.
func LogPath(serverDir string) string {
	return filepath.Join(StateDir(serverDir), logFileName)
}

// Running reports whether a server directory has a live supervisor.
func Running(serverDir string) bool {
	pid, err := shared.ReadLockPIDIn(serverDir)
	if err != nil {
		return false
	}
	return shared.IsPIDRunning(pid)
}

// PID is the process id of a server's supervisor, or 0 when it is not running.
func PID(serverDir string) int {
	pid, err := shared.ReadLockPIDIn(serverDir)
	if err != nil || !shared.IsPIDRunning(pid) {
		return 0
	}
	return pid
}

// request is one line of the control protocol. Replies are the same shape, so
// both ends encode a single struct per line.
type request struct {
	Op  string `json:"op"`
	Arg string `json:"arg,omitempty"`
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// The operations a client may ask of a supervisor.
const (
	opExec = "exec"
	opStop = "stop"
	opKill = "kill"
	opPing = "ping"
)

// Start launches the detached supervisor for a server directory. It returns as
// soon as the supervisor is spawned: everything the start does or fails at is
// written to the server's log, which is what the caller is already reading.
//
// name is what the server is called in its own log; the folder name is used
// when it is empty.
func Start(serverDir string, name string) error {
	dir, err := filepath.Abs(serverDir)
	if err != nil {
		return err
	}

	if Running(dir) {
		return ErrAlreadyRunning
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the osmium executable: %w", err)
	}

	// The supervisor keeps no terminal of its own — its output is the log file
	// — so its standard streams go nowhere. Leaving them attached would tie it
	// to the terminal that launched it and corrupt the TUI's screen.
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(exe, "serve", "--dir", dir, "--name", name)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	// Let it go: the supervisor outlives this process, so it must not be left
	// as a child nobody will ever wait on.
	return cmd.Process.Release()
}

// send performs one request/reply exchange with a server's supervisor.
func send(serverDir string, req request) error {
	conn, err := dial(serverDir)
	if err != nil {
		return ErrNotRunning
	}
	defer conn.Close()

	line, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(conn, "%s\n", line); err != nil {
		return err
	}

	var res reply
	if err := json.NewDecoder(conn).Decode(&res); err != nil {
		// A supervisor that hangs up without answering has taken the request
		// and gone away — stopping does exactly that.
		return nil
	}
	if !res.OK && res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

// Exec sends one console command to a running server.
func Exec(serverDir, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	return send(serverDir, request{Op: opExec, Arg: command})
}

// Stop asks a server to shut down. A graceful stop lets it save its world;
// force kills the process outright.
func Stop(serverDir string, force bool) error {
	op := opStop
	if force {
		op = opKill
	}
	return send(serverDir, request{Op: op})
}

// Ping reports whether a supervisor is listening and answering.
func Ping(serverDir string) error {
	return send(serverDir, request{Op: opPing})
}

// maxSocketPath is how long a unix socket path may be. The limit is a fixed
// buffer in the kernel — 108 bytes on Linux, 104 on macOS — and it is short
// enough that a server sitting a few folders deep can exceed it.
const maxSocketPath = 100

// socketPath is where a server's control socket lives.
//
// It belongs in the server's own folder, which is what keeps two servers from
// colliding and lets the socket die with the files it belongs to. A folder
// whose path is too long for a socket address gets one in the user's runtime
// directory instead, named after a hash of the folder so both ends work it out
// without having to agree on anything but the path they already share.
func socketPath(serverDir string) string {
	dir := absoluteDir(serverDir)

	if inFolder := filepath.Join(StateDir(dir), sockFileName); len(inFolder) <= maxSocketPath {
		return inFolder
	}
	return filepath.Join(runtimeDir(), fingerprint(dir)+".sock")
}

// absoluteDir normalises a server directory, so a supervisor started with a
// relative path and a client using an absolute one still meet on the same
// socket.
func absoluteDir(serverDir string) string {
	if serverDir == "" {
		serverDir = "."
	}
	if absolute, err := filepath.Abs(serverDir); err == nil {
		return absolute
	}
	return serverDir
}

// fingerprint identifies a server folder in a fixed number of characters.
func fingerprint(dir string) string {
	sum := sha1.Sum([]byte(dir))
	return hex.EncodeToString(sum[:8])
}

// runtimeDir is the per-user directory the fallback sockets live in.
func runtimeDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "osmium")
}

func portPath(serverDir string) string {
	return filepath.Join(StateDir(serverDir), portFileName)
}
