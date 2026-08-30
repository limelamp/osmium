package expose

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Opening a port on the router is done over UPnP's Internet Gateway Device
// profile, which is what home routers have spoken for twenty years and what
// the game's own LAN features use. Two things are asked of it: what the
// router's outside address is, and that traffic arriving on a port be sent to
// this machine.
//
// It is deliberately all here and all standard library. A port mapping is a
// change to somebody's network, so it should be readable in one sitting rather
// than delegated to a dependency nobody reads.

// discoveryWait is how long the router is given to answer a search.
const discoveryWait = 3 * time.Second

// gateway is a router that has answered, and the service that maps ports.
type gateway struct {
	controlURL string
	service    string
}

// Mapping is one port opened on the router.
type Mapping struct {
	External string
	Port     int
	Until    time.Time
}

// Open asks the router to send a port through to this machine.
//
// The mapping is described with the server's name so that anyone looking at
// the router's list can see what it is for and remove it themselves, and it is
// leased for a day rather than for ever: a machine that goes away without
// tidying up should not leave a hole open indefinitely.
func Open(ctx context.Context, port int, name string) (Mapping, error) {
	router, err := find(ctx)
	if err != nil {
		return Mapping{}, err
	}

	local := LocalAddress()
	if local == "" {
		return Mapping{}, fmt.Errorf("this machine has no address on the local network")
	}

	const lease = 24 * 60 * 60

	body := fmt.Sprintf(`<NewRemoteHost></NewRemoteHost>`+
		`<NewExternalPort>%d</NewExternalPort>`+
		`<NewProtocol>TCP</NewProtocol>`+
		`<NewInternalPort>%d</NewInternalPort>`+
		`<NewInternalClient>%s</NewInternalClient>`+
		`<NewEnabled>1</NewEnabled>`+
		`<NewPortMappingDescription>Osmium: %s</NewPortMappingDescription>`+
		`<NewLeaseDuration>%d</NewLeaseDuration>`,
		port, port, local, xmlEscape(name), lease)

	if _, err := router.call(ctx, "AddPortMapping", body); err != nil {
		return Mapping{}, fmt.Errorf("the router refused to open %d: %w", port, err)
	}

	external, err := router.externalAddress(ctx)
	if err != nil {
		external = ""
	}

	return Mapping{External: external, Port: port, Until: time.Now().Add(lease * time.Second)}, nil
}

// Close takes a mapping back.
func Close(ctx context.Context, port int) error {
	router, err := find(ctx)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(`<NewRemoteHost></NewRemoteHost>`+
		`<NewExternalPort>%d</NewExternalPort>`+
		`<NewProtocol>TCP</NewProtocol>`, port)

	if _, err := router.call(ctx, "DeletePortMapping", body); err != nil {
		return fmt.Errorf("the router refused to close %d: %w", port, err)
	}
	return nil
}

// ExternalAddress is the address the world sees this network as, according to
// the router itself.
func ExternalAddress(ctx context.Context) (string, error) {
	router, err := find(ctx)
	if err != nil {
		return "", err
	}
	return router.externalAddress(ctx)
}

// externalAddress asks the gateway for its outside address.
func (g gateway) externalAddress(ctx context.Context) (string, error) {
	answer, err := g.call(ctx, "GetExternalIPAddress", "")
	if err != nil {
		return "", err
	}

	found := externalPattern.FindStringSubmatch(answer)
	if found == nil {
		return "", fmt.Errorf("the router did not give an address")
	}
	return found[1], nil
}

var externalPattern = regexp.MustCompile(`<NewExternalIPAddress>([^<]*)</NewExternalIPAddress>`)

// find discovers the router and the service that maps ports.
func find(ctx context.Context) (gateway, error) {
	locations, err := search(ctx)
	if err != nil {
		return gateway{}, err
	}

	for _, location := range locations {
		if router, err := describe(ctx, location); err == nil {
			return router, nil
		}
	}

	return gateway{}, fmt.Errorf("no router here answers UPnP; the port would have to be forwarded by hand")
}

// search sends the discovery request every UPnP router listens for.
func search(ctx context.Context) ([]string, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	target := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

	request := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n"

	if _, err := conn.WriteTo([]byte(request), target); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(discoveryWait)
	if until, ok := ctx.Deadline(); ok && until.Before(deadline) {
		deadline = until
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	var locations []string
	buffer := make([]byte, 2048)

	for {
		read, _, err := conn.ReadFrom(buffer)
		if err != nil {
			break
		}

		if location := header(string(buffer[:read]), "LOCATION"); location != "" {
			locations = append(locations, location)
		}
	}

	if len(locations) == 0 {
		return nil, fmt.Errorf("no router answered")
	}
	return locations, nil
}

// header picks one header out of an SSDP answer.
func header(answer, name string) string {
	for _, line := range strings.Split(answer, "\r\n") {
		key, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(key), name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// device is the part of a router's description this needs.
type device struct {
	Device struct {
		DeviceList struct {
			Devices []subDevice `xml:"device"`
		} `xml:"deviceList"`
		ServiceList struct {
			Services []service `xml:"service"`
		} `xml:"serviceList"`
	} `xml:"device"`
}

type subDevice struct {
	DeviceList struct {
		Devices []subDevice `xml:"device"`
	} `xml:"deviceList"`
	ServiceList struct {
		Services []service `xml:"service"`
	} `xml:"serviceList"`
}

type service struct {
	Type       string `xml:"serviceType"`
	ControlURL string `xml:"controlURL"`
}

// describe reads a router's description and finds the port-mapping service,
// which is nested two devices deep in every implementation.
func describe(ctx context.Context, location string) (gateway, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, http.NoBody)
	if err != nil {
		return gateway{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return gateway{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return gateway{}, err
	}

	var described device
	if err := xml.Unmarshal(body, &described); err != nil {
		return gateway{}, err
	}

	base, err := url.Parse(location)
	if err != nil {
		return gateway{}, err
	}

	services := described.Device.ServiceList.Services
	for _, child := range described.Device.DeviceList.Devices {
		services = append(services, collect(child)...)
	}

	for _, found := range services {
		if !strings.Contains(found.Type, "WANIPConnection") && !strings.Contains(found.Type, "WANPPPConnection") {
			continue
		}

		control, err := base.Parse(found.ControlURL)
		if err != nil {
			continue
		}

		return gateway{controlURL: control.String(), service: found.Type}, nil
	}

	return gateway{}, fmt.Errorf("this router does not offer port mapping")
}

// collect gathers the services of a device and everything under it.
func collect(from subDevice) []service {
	services := from.ServiceList.Services
	for _, child := range from.DeviceList.Devices {
		services = append(services, collect(child)...)
	}
	return services
}

// call makes one SOAP request to the router.
func (g gateway) call(ctx context.Context, action, body string) (string, error) {
	envelope := `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>` +
		fmt.Sprintf(`<u:%s xmlns:u=%q>%s</u:%s>`, action, g.service, body, action) +
		`</s:Body></s:Envelope>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.controlURL,
		bytes.NewReader([]byte(envelope)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, g.service, action))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s (%s)", resp.Status, faultOf(string(answer)))
	}
	return string(answer), nil
}

var faultPattern = regexp.MustCompile(`<errorDescription>([^<]*)</errorDescription>`)

// faultOf pulls the reason out of a SOAP fault.
func faultOf(body string) string {
	if found := faultPattern.FindStringSubmatch(body); found != nil {
		return found[1]
	}
	return "no reason given"
}

// xmlEscape makes a name safe to put in a request.
func xmlEscape(text string) string {
	var out bytes.Buffer
	if err := xml.EscapeText(&out, []byte(text)); err != nil {
		return ""
	}
	return out.String()
}

// PortOf reads a port out of an address, for a caller that has one string.
func PortOf(address string) int {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0
	}

	value, _ := strconv.Atoi(port)
	return value
}
