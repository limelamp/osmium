// Package expose answers the question every new server owner asks: how does
// anyone else get in?
//
// It works out the addresses that reach a server, checks whether they actually
// do, and — when asked — has the router open the port itself. Nothing here
// installs anything or changes this machine's configuration: the port mapping
// is a request to the router over protocols it already speaks, and it can be
// taken back as easily as it was made.
package expose

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/mcping"
)

// Where a server can be reached from.
type Where int

// The three distances that matter.
const (
	ThisMachine Where = iota
	SameNetwork
	Internet
)

// Name is what to call one of them.
func (w Where) Name() string {
	switch w {
	case SameNetwork:
		return "same network"
	case Internet:
		return "internet"
	default:
		return "this machine"
	}
}

// Address is one way in, and whether it works.
type Address struct {
	Where   Where
	Host    string
	Port    int
	Checked bool
	Answers bool
	Note    string
}

// String is the address as a player would type it.
func (a Address) String() string {
	if a.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// Addresses are the ways into a server, nearest first.
//
// The public address comes from the router rather than from a third party
// where the router will say: asking the thing in the room what its address is
// beats telling a website about your server to find out.
func Addresses(ctx context.Context, port int) []Address {
	found := []Address{
		{Where: ThisMachine, Host: "localhost", Port: port},
	}

	if lan := LocalAddress(); lan != "" {
		found = append(found, Address{Where: SameNetwork, Host: lan, Port: port})
	}

	public := Address{Where: Internet, Port: port}
	if external, err := ExternalAddress(ctx); err == nil {
		public.Host = external
	} else {
		public.Note = "the router would not say (" + short(err) + ")"
	}
	found = append(found, public)

	return found
}

// Check asks each address whether a server actually answers on it.
//
// The internet address is checked from this machine, which is not the same as
// checking it from outside: many routers refuse to loop a connection back to
// themselves even when the port is open. A failure there is reported as
// undecided rather than as closed, because saying "closed" when it is not is
// worse than saying nothing.
func Check(ctx context.Context, addresses []Address) []Address {
	checked := make([]Address, 0, len(addresses))

	for _, address := range addresses {
		if address.Host == "" {
			checked = append(checked, address)
			continue
		}

		_, err := mcping.Ping(ctx, address.Host, address.Port, 2*time.Second)

		address.Checked = true
		address.Answers = err == nil

		if err != nil && address.Where == Internet {
			address.Note = "no answer from here — many routers refuse to loop back, " +
				"so this does not prove it is closed"
		}
		if err != nil && address.Where != Internet {
			address.Note = short(err)
		}

		checked = append(checked, address)
	}

	return checked
}

// LocalAddress is this machine's address on its own network.
func LocalAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, address := range addresses {
			ipnet, ok := address.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}

			if ip := ipnet.IP.To4(); ip != nil && ip.IsPrivate() {
				return ip.String()
			}
		}
	}

	return ""
}

// short is an error reduced to something that fits on a line.
func short(err error) string {
	text := err.Error()
	if cut := strings.LastIndex(text, ": "); cut > 0 && len(text) > 60 {
		text = text[cut+2:]
	}
	return text
}
