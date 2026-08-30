package shared

import (
	"context"
	"time"

	"github.com/limelamp/osmium/internal/mcping"
	"github.com/limelamp/osmium/internal/properties"
)

// Live is what a server says about itself right now.
type Live struct {
	Version string
	MOTD    string
	Online  int
	Max     int
	Names   []string
	Latency time.Duration
	Up      bool
}

// pingWait is how long a server is given to answer. It is short because this
// runs on a timer behind an interface: a server that is too busy to answer in
// two seconds will be asked again in five.
const pingWait = 2 * time.Second

// Ask reads a server's live state the way the game's own server list does.
//
// Nothing is typed into the server to get this. Osmium used to send "list"
// every second to find out who was on, and every answer landed in the console
// among the server's own output — so the log of what happened was mostly
// Osmium talking to itself. The status port answers the same question and the
// console never hears about it.
func Ask(dir string) Live {
	props, err := properties.Read(dir)
	if err != nil {
		return Live{}
	}

	port := props.Port()
	if port == 0 {
		return Live{}
	}

	host := props.Get(properties.KeyAddress)
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingWait)
	defer cancel()

	status, err := mcping.Ping(ctx, host, port, pingWait)
	if err != nil {
		return Live{}
	}

	return Live{
		Version: status.Version,
		MOTD:    status.MOTD,
		Online:  status.Online,
		Max:     status.Max,
		Names:   status.Sample,
		Latency: status.Latency,
		Up:      true,
	}
}

// Named reports whether the status carries the names of who is on.
//
// A server samples its player list rather than sending all of it, and some
// hide it altogether, so a count without names is a normal answer and not a
// failure. Anything showing players has to be able to say "three online" when
// that is all it knows.
func (l Live) Named() bool { return len(l.Names) > 0 }
