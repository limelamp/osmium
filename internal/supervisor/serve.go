package supervisor

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/util"
)

// Serve runs the supervisor for one server directory: it starts the server,
// records everything it says into the log, and answers control requests until
// the server exits. It is the body of the hidden "osmium serve" command and is
// never run in the foreground of a session the user can see.
func Serve(serverDir, name string) error {
	dir, err := filepath.Abs(serverDir)
	if err != nil {
		return err
	}
	if name == "" {
		name = filepath.Base(dir)
	}

	// The log is opened first so that everything below, failures included, has
	// somewhere to be seen: the client that started this supervisor is already
	// reading the file.
	log, err := OpenLog(dir)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()

	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		log.Write(TagError, "could not read "+config.OsmiumFileName+" in "+dir)
		return err
	}

	// The lock refuses a second supervisor for the same folder, which is what
	// keeps two Osmium sessions from starting the same server twice.
	if err := shared.WriteLockPIDIn(dir, os.Getpid()); err != nil {
		log.Write(TagError, err.Error())
		return err
	}
	defer func() { _ = shared.RemoveLockFileIn(dir) }()

	javaPath, args := util.GetServerRunCommand(dir, conf.Loader, conf.AllocatedRAM)

	cmd := exec.Command(javaPath, args...)
	cmd.Dir = dir
	cmd.Stdout = log.Writer(TagServer)
	cmd.Stderr = log.Writer(TagServer)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Write(TagError, err.Error())
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Write(TagError, "could not start the server: "+err.Error())
		return err
	}
	log.Write(TagOsmium, fmt.Sprintf("- starting %s (%s %s) -", name, conf.Loader, conf.Version))

	listener, err := listen(dir)
	if err != nil {
		log.Write(TagError, "could not open the control socket: "+err.Error())
		_ = cmd.Process.Kill()
		return err
	}
	defer cleanupSocket(dir)

	control := &controller{log: log, stdin: stdin, cmd: cmd, name: name}
	go control.accept(listener)
	go control.handleSignals()

	err = cmd.Wait()
	log.Write(TagOsmium, "- "+name+" exited"+exitDetail(err)+" -")

	// Closing the listener ends the accept loop, so nothing is left listening
	// on a socket whose server is gone.
	_ = listener.Close()
	return nil
}

// exitDetail describes how the server process ended, when it ended badly.
func exitDetail(err error) string {
	if err == nil {
		return ""
	}
	exit := &exec.ExitError{}
	if errors.As(err, &exit) {
		return fmt.Sprintf(" with status %d", exit.ExitCode())
	}
	return ": " + err.Error()
}

// controller answers control requests for one running server.
type controller struct {
	log   *Log
	stdin io.WriteCloser
	cmd   *exec.Cmd
	name  string
}

// accept serves control connections until the listener is closed.
func (c *controller) accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // the server exited and the listener was closed
		}
		go c.handle(conn)
	}
}

// handle reads one request per line and answers it.
func (c *controller) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			c.answer(conn, err)
			continue
		}
		c.answer(conn, c.apply(req))
	}
}

// apply carries out one request against the running server.
func (c *controller) apply(req request) error {
	switch req.Op {
	case opPing:
		return nil

	case opExec:
		c.log.Write(TagYou, "› "+req.Arg)
		_, err := fmt.Fprintln(c.stdin, req.Arg)
		return err

	case opStop:
		// "stop" lets the server save its world and shut down properly; a kill
		// is what the caller asks for explicitly when it will not.
		c.log.Write(TagOsmium, "- stopping "+c.name+" -")
		_, err := fmt.Fprintln(c.stdin, "stop")
		return err

	case opKill:
		c.log.Write(TagOsmium, "- killing "+c.name+" -")
		if c.cmd.Process == nil {
			return nil
		}
		return c.cmd.Process.Kill()
	}

	return fmt.Errorf("unknown operation %q", req.Op)
}

// answer writes the reply for a request.
func (c *controller) answer(conn net.Conn, err error) {
	res := reply{OK: err == nil}
	if err != nil {
		res.Error = err.Error()
	}

	if encoded, marshalErr := json.Marshal(res); marshalErr == nil {
		fmt.Fprintf(conn, "%s\n", encoded)
	}
}

// handleSignals shuts the server down properly when the machine asks the
// supervisor to quit, instead of leaving a Minecraft world half-saved.
func (c *controller) handleSignals() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	<-signals
	c.log.Write(TagOsmium, "- shutting "+c.name+" down -")
	fmt.Fprintln(c.stdin, "stop")
}
