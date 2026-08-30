// Package mcping asks a Minecraft server about itself the way the game's own
// server list does.
//
// Every server answers this, on the port it already listens on, with no
// configuration and no plugin: the version it runs, its message of the day,
// how many players are on and a sample of who they are. That makes it the one
// source of live information that costs the server console nothing — which is
// why Osmium uses it instead of typing "list" into the server every second and
// filling the log with its own questions.
package mcping

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Status is what a server says about itself.
type Status struct {
	Version     string
	Protocol    int
	MOTD        string
	Online      int
	Max         int
	Sample      []string
	Latency     time.Duration
	EnforcesTLS bool
}

// status is the JSON a server answers with.
type status struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
		Sample []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"sample"`
	} `json:"players"`
	Description json.RawMessage `json:"description"`
}

// Ping asks a server for its status.
//
// The exchange is the handshake and status request of the Minecraft protocol,
// which has not changed since 1.7 — one of the few things about the game that
// can be relied on across versions.
func Ping(ctx context.Context, host string, port int, wait time.Duration) (Status, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	if wait <= 0 {
		wait = 2 * time.Second
	}

	dialer := net.Dialer{Timeout: wait}
	address := net.JoinHostPort(host, strconv.Itoa(port))

	started := time.Now()

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Status{}, err
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(wait)); err != nil {
		return Status{}, err
	}

	// The handshake: protocol version -1 means "I am only asking", which every
	// server accepts whatever version it runs.
	handshake := packet(0x00,
		varint(-1),
		text(host),
		port16(port),
		varint(1), // next state: status
	)

	if _, err := conn.Write(handshake); err != nil {
		return Status{}, err
	}
	if _, err := conn.Write(packet(0x00)); err != nil {
		return Status{}, err
	}

	reader := bufio.NewReader(conn)

	length, err := readVarint(reader)
	if err != nil {
		return Status{}, fmt.Errorf("the server did not answer a status request: %w", err)
	}
	if length <= 0 || length > 4<<20 {
		return Status{}, fmt.Errorf("the server answered with %d bytes", length)
	}

	id, err := readVarint(reader)
	if err != nil {
		return Status{}, err
	}
	if id != 0x00 {
		return Status{}, fmt.Errorf("the server answered with packet %d", id)
	}

	body, err := readString(reader)
	if err != nil {
		return Status{}, err
	}

	var answer status
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		return Status{}, fmt.Errorf("the server's answer is not readable: %w", err)
	}

	found := Status{
		Version:  answer.Version.Name,
		Protocol: answer.Version.Protocol,
		Online:   answer.Players.Online,
		Max:      answer.Players.Max,
		MOTD:     describe(answer.Description),
		Latency:  time.Since(started),
	}

	for _, player := range answer.Players.Sample {
		if player.Name != "" {
			found.Sample = append(found.Sample, player.Name)
		}
	}

	return found, nil
}

// describe flattens a message of the day, which a server may write as a plain
// string or as a tree of coloured parts.
func describe(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return strip(plain)
	}

	var component struct {
		Text  string            `json:"text"`
		Extra []json.RawMessage `json:"extra"`
	}
	if err := json.Unmarshal(raw, &component); err != nil {
		return ""
	}

	var out strings.Builder
	out.WriteString(component.Text)
	for _, part := range component.Extra {
		out.WriteString(describe(part))
	}

	return strip(out.String())
}

// strip removes the section-sign colour codes a message of the day carries.
func strip(text string) string {
	var out strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '§' && i+1 < len(runes) {
			i++
			continue
		}
		out.WriteRune(runes[i])
	}

	return strings.TrimSpace(out.String())
}

// packet frames a payload with its length, as the protocol wants it.
func packet(id byte, parts ...[]byte) []byte {
	body := append([]byte{}, varint(int(id))...)
	for _, part := range parts {
		body = append(body, part...)
	}

	return append(varint(len(body)), body...)
}

// varint is the protocol's variable-length integer.
func varint(value int) []byte {
	var out []byte

	unsigned := uint32(value) //nolint:gosec // the protocol's own two's-complement encoding
	for {
		piece := byte(unsigned & 0x7F)
		unsigned >>= 7

		if unsigned != 0 {
			piece |= 0x80
		}

		out = append(out, piece)
		if unsigned == 0 {
			return out
		}
	}
}

// text is a length-prefixed string.
func text(value string) []byte {
	return append(varint(len(value)), value...)
}

// port16 is an unsigned short, big endian.
func port16(port int) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(port)) //nolint:gosec // a port is sixteen bits by definition
	return out
}

// readVarint reads one variable-length integer.
func readVarint(reader io.ByteReader) (int, error) {
	var value, shift int

	for range 5 {
		piece, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		value |= int(piece&0x7F) << shift
		if piece&0x80 == 0 {
			return value, nil
		}

		shift += 7
	}

	return 0, fmt.Errorf("a number in the answer is malformed")
}

// readString reads a length-prefixed string.
func readString(reader *bufio.Reader) (string, error) {
	length, err := readVarint(reader)
	if err != nil {
		return "", err
	}
	if length < 0 || length > 4<<20 {
		return "", fmt.Errorf("the answer claims to be %d bytes", length)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return "", err
	}

	return string(body), nil
}
