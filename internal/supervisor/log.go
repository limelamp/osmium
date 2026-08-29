package supervisor

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
)

// Tag says where a log line came from. Every line carries one, so the reader
// can keep server output, Osmium's own progress and typed commands apart
// instead of showing one indistinguishable stream.
type Tag string

const (
	// TagServer is output from the Minecraft server process itself.
	TagServer Tag = "server"
	// TagOsmium is Osmium's own notices: starting, stopping, exiting.
	TagOsmium Tag = "osmium"
	// TagYou is a console command someone typed.
	TagYou Tag = "you"
	// TagMod is progress from a mod or plugin operation.
	TagMod Tag = "mod"
	// TagError is a failure, wherever it came from.
	TagError Tag = "error"
)

// Line is one tagged log line.
type Line struct {
	Tag  Tag
	Text string
}

// The log file stores the tag and the text separated by a tab, which no
// terminal output of a Minecraft server contains at the start of a line.
const tagSeparator = "\t"

// Encode renders a line for the log file.
func (l Line) Encode() string {
	return string(l.Tag) + tagSeparator + l.Text
}

// ParseLine reads back a line written by Encode. A line without a tag - a log
// file written by something else, or a half-written line - is server output.
func ParseLine(raw string) Line {
	tag, text, found := strings.Cut(raw, tagSeparator)
	if !found {
		return Line{Tag: TagServer, Text: raw}
	}
	return Line{Tag: Tag(tag), Text: text}
}

// maxLogBytes is how large a server log may grow before it is rotated, so a
// server left running for weeks cannot fill the disk.
const maxLogBytes = 4 << 20

// Log is the supervisor's writer for one server's log file. Both the process
// output goroutines and the socket handlers write to it, so it is mutexed.
type Log struct {
	mu   sync.Mutex
	file *os.File
	path string
	size int64
}

// OpenLog opens (or creates) a server's log file for appending.
func OpenLog(serverDir string) (*Log, error) {
	if err := os.MkdirAll(StateDir(serverDir), 0o750); err != nil {
		return nil, err
	}

	path := LogPath(serverDir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	size := int64(0)
	if info, err := file.Stat(); err == nil {
		size = info.Size()
	}

	return &Log{file: file, path: path, size: size}, nil
}

// Write appends one tagged line.
func (l *Log) Write(tag Tag, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write(Line{Tag: tag, Text: text})
}

// write appends a line and rotates the file when it has grown too large. The
// caller holds the mutex.
func (l *Log) write(line Line) {
	if l.file == nil {
		return
	}

	n, err := l.file.WriteString(line.Encode() + "\n")
	if err != nil {
		return
	}

	l.size += int64(n)
	if l.size >= maxLogBytes {
		l.rotate()
	}
}

// rotate moves the current log aside and starts an empty one, keeping exactly
// one previous file.
func (l *Log) rotate() {
	_ = l.file.Close()
	_ = os.Rename(l.path, l.path+".1")

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		l.file = nil
		return
	}
	l.file, l.size = file, 0
}

// Close releases the log file.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// Writer returns an io.Writer that records everything written to it under one
// tag, which is how the server process's stdout and stderr reach the log.
func (l *Log) Writer(tag Tag) io.Writer {
	return &tagWriter{log: l, tag: tag}
}

// tagWriter splits a byte stream into lines and tags each one. A process writes
// in arbitrary chunks, so a partial line is held back until its newline
// arrives rather than being logged as a line of its own.
type tagWriter struct {
	log     *Log
	tag     Tag
	partial []byte
}

func (w *tagWriter) Write(p []byte) (int, error) {
	w.log.mu.Lock()
	defer w.log.mu.Unlock()

	w.partial = append(w.partial, p...)

	for {
		index := bytes.IndexByte(w.partial, '\n')
		if index < 0 {
			break
		}

		text := strings.TrimRight(string(w.partial[:index]), "\r")
		w.partial = w.partial[index+1:]
		w.log.write(Line{Tag: w.tag, Text: text})
	}

	return len(p), nil
}

// tailChunk is how much of the end of a log file is read to recover the last
// lines, which is plenty for a few thousand lines of server output.
const tailChunk = 512 << 10

// ReadLast returns the last n lines of a log file, oldest first. A missing log
// file is not an error: the supervisor may not have written to it yet.
func ReadLast(path string, n int) ([]Line, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	offset := max(info.Size()-tailChunk, 0)
	data := make([]byte, info.Size()-offset)
	if _, err := file.ReadAt(data, offset); err != nil && err != io.EOF {
		return nil, err
	}

	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil, nil
	}

	raw := strings.Split(text, "\n")
	if offset > 0 && len(raw) > 0 {
		// The first line of a mid-file read is almost certainly cut in half.
		raw = raw[1:]
	}
	if len(raw) > n {
		raw = raw[len(raw)-n:]
	}

	lines := make([]Line, 0, len(raw))
	for _, one := range raw {
		lines = append(lines, ParseLine(one))
	}
	return lines, nil
}

// Tailer follows a server's log file, handing back whatever has been appended
// since it was last asked. It is what lets a TUI opened long after a server
// started show that server's output as it arrives.
type Tailer struct {
	path   string
	offset int64
	buffer []byte
}

// NewTailer starts following a log file. history is how many existing lines to
// return from the first Read, so attaching to a running server shows the
// context it is already in rather than an empty pane.
func NewTailer(path string, history int) (*Tailer, []Line) {
	lines, _ := ReadLast(path, history)

	tailer := &Tailer{path: path}
	if info, err := os.Stat(path); err == nil {
		tailer.offset = info.Size()
	}
	return tailer, lines
}

// Read returns the lines appended since the last call. It tolerates a log file
// that does not exist yet, and starts over when one is rotated out from under
// it.
func (t *Tailer) Read() []Line {
	file, err := os.Open(t.path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil
	}

	// A file that shrank was rotated, so what is there now is all new.
	if info.Size() < t.offset {
		t.offset, t.buffer = 0, nil
	}
	if info.Size() == t.offset {
		return nil
	}

	data := make([]byte, info.Size()-t.offset)
	read, err := file.ReadAt(data, t.offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	t.offset += int64(read)

	// Only whole lines are returned; a line the supervisor is halfway through
	// writing waits for the rest of itself.
	t.buffer = append(t.buffer, data[:read]...)
	index := bytes.LastIndexByte(t.buffer, '\n')
	if index < 0 {
		return nil
	}

	complete := string(t.buffer[:index])
	t.buffer = t.buffer[index+1:]

	var lines []Line
	for _, raw := range strings.Split(complete, "\n") {
		lines = append(lines, ParseLine(raw))
	}
	return lines
}
