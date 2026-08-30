// Package health checks a server for the things that waste an evening: the
// wrong Java, more memory promised than the machine has, a port already in
// use, an unaccepted licence, files that are supposed to be there and are not.
//
// Everything here is a look, never a change. A check that fixed things would
// have to be trusted; one that only reports can be run on anything.
package health

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/properties"
	"github.com/limelamp/osmium/internal/supervisor"
)

// Level is how much a finding matters.
type Level int

// The levels, in the order they are worth reading.
const (
	OK Level = iota
	Note
	Problem
)

// Mark is the character a level is drawn with.
func (l Level) Mark() string {
	switch l {
	case Problem:
		return "✕"
	case Note:
		return "!"
	default:
		return "✓"
	}
}

// Finding is one thing that was looked at.
type Finding struct {
	Level  Level
	Title  string
	Detail string
}

// Line is a finding written out.
func (f Finding) Line() string {
	line := f.Level.Mark() + " " + f.Title
	if f.Detail != "" {
		line += "   " + f.Detail
	}
	return line
}

// Check looks a server over and reports what it finds.
func Check(dir string) []Finding {
	var findings []Finding

	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		return []Finding{{
			Level:  Problem,
			Title:  "no " + config.OsmiumFileName + " here",
			Detail: "this folder is not an Osmium server",
		}}
	}

	findings = append(
		findings,
		checkJava(conf.Version),
		checkMemory(conf.AllocatedRAM),
		checkEula(dir),
		checkPort(dir),
	)

	findings = append(findings, checkFiles(dir)...)

	return findings
}

// checkJava compares the Java on the path with what this Minecraft version
// needs. It is the single most common reason a server will not start.
func checkJava(version string) Finding {
	needed := javaFor(version)

	found, err := javaVersion()
	if err != nil {
		return Finding{Level: Problem, Title: "java", Detail: "not found on the path"}
	}

	switch {
	case needed == 0:
		return Finding{Title: "java", Detail: fmt.Sprintf("java %d", found)}

	case found < needed:
		return Finding{
			Level: Problem, Title: "java",
			Detail: fmt.Sprintf("java %d found, Minecraft %s needs %d or newer", found, version, needed),
		}

	default:
		return Finding{Title: "java", Detail: fmt.Sprintf("java %d, correct for %s", found, version)}
	}
}

// javaFor is the Java version a Minecraft version needs.
//
// Mojang has moved this three times, and every move stranded servers whose
// owners had no idea why the jar simply exited.
func javaFor(version string) int {
	major, minor := minecraftVersion(version)
	if major != 1 {
		return 0
	}

	switch {
	case minor >= 21:
		return 21
	case minor >= 20:
		return 17 // 1.20.5 moved to 21, which the check below narrows
	case minor >= 18:
		return 17
	case minor == 17:
		return 16
	case minor > 0:
		return 8
	}
	return 0
}

// minecraftVersion splits "1.21.1" into its first two numbers.
func minecraftVersion(version string) (major, minor int) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return 0, 0
	}

	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	return major, minor
}

// javaVersionPattern picks the number out of what java prints about itself,
// which is on standard error and has changed shape twice.
var javaVersionPattern = regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)

// javaVersion asks the Java on the path what it is.
func javaVersion() (int, error) {
	out, err := exec.Command("java", "-version").CombinedOutput() //nolint:gosec,noctx // a fixed command, no input
	if err != nil {
		return 0, err
	}

	match := javaVersionPattern.FindStringSubmatch(string(out))
	if match == nil {
		return 0, fmt.Errorf("java did not say what version it is")
	}

	major, _ := strconv.Atoi(match[1])
	if major == 1 && len(match) > 2 {
		// The old "1.8.0" form, where the number that matters is the second.
		minor, _ := strconv.Atoi(match[2])
		return minor, nil
	}
	return major, nil
}

// checkMemory compares what the server is promised with what the machine has.
func checkMemory(allocated string) Finding {
	wanted := megabytes(allocated)
	if wanted == 0 {
		return Finding{Level: Note, Title: "memory", Detail: "no allocation recorded"}
	}

	total := systemMemory()
	if total == 0 {
		return Finding{Title: "memory", Detail: allocated + " allocated"}
	}

	switch {
	case wanted > total:
		return Finding{
			Level: Problem, Title: "memory",
			Detail: fmt.Sprintf("%s allocated, the machine has %s", allocated, formatMB(total)),
		}

	case float64(wanted) > float64(total)*0.8:
		return Finding{
			Level: Note, Title: "memory",
			Detail: fmt.Sprintf("%s of the machine's %s — little left for anything else",
				allocated, formatMB(total)),
		}

	default:
		return Finding{Title: "memory", Detail: fmt.Sprintf("%s of %s", allocated, formatMB(total))}
	}
}

// megabytes reads "4G" or "3072M" as a number of megabytes.
func megabytes(allocated string) int {
	value := strings.ToUpper(strings.TrimSpace(allocated))
	if value == "" {
		return 0
	}

	unit := value[len(value)-1]
	number, err := strconv.Atoi(strings.TrimRight(value, "GM"))
	if err != nil {
		return 0
	}

	if unit == 'G' {
		return number * 1024
	}
	return number
}

// formatMB renders a megabyte count.
func formatMB(mb int) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

// systemMemory is how much memory the machine has, in megabytes, or zero where
// that cannot be asked without leaving the standard library.
func systemMemory() int {
	switch runtime.GOOS {
	case "linux":
		file, err := os.Open("/proc/meminfo")
		if err != nil {
			return 0
		}
		defer func() { _ = file.Close() }()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 && fields[0] == "MemTotal:" {
				kb, _ := strconv.Atoi(fields[1])
				return kb / 1024
			}
		}

	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output() //nolint:gosec,noctx // a fixed command
		if err != nil {
			return 0
		}

		bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		return int(bytes / (1024 * 1024))
	}

	return 0
}

// checkEula reports whether the licence has been accepted, without which a
// server writes one line to its log and exits.
func checkEula(dir string) Finding {
	data, err := os.ReadFile(filepath.Join(dir, "eula.txt"))
	if err != nil {
		return Finding{Level: Note, Title: "eula", Detail: "not accepted yet"}
	}

	if strings.Contains(strings.ToLower(string(data)), "eula=true") {
		return Finding{Title: "eula", Detail: "accepted"}
	}
	return Finding{Level: Problem, Title: "eula", Detail: "eula.txt says false"}
}

// checkPort reports whether the port this server wants is free, or held by
// this server itself.
func checkPort(dir string) Finding {
	props, err := properties.Read(dir)
	if err != nil {
		return Finding{Level: Note, Title: "port", Detail: "no server.properties yet"}
	}

	port := props.Port()
	if port == 0 {
		return Finding{Level: Note, Title: "port", Detail: "not set"}
	}

	if supervisor.Running(dir) {
		return Finding{Title: "port", Detail: fmt.Sprintf("%d, in use by this server", port)}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return Finding{
			Level: Problem, Title: "port",
			Detail: fmt.Sprintf("%d is already in use by something else", port),
		}
	}
	_ = listener.Close()

	return Finding{Title: "port", Detail: fmt.Sprintf("%d is free", port)}
}

// checkFiles reports what the lock says should be installed and is not, and
// what is installed that nothing accounts for.
func checkFiles(dir string) []Finding {
	server, err := pack.Open(dir)
	if err != nil {
		return nil
	}

	var findings []Finding
	for _, kind := range []config.Kind{config.Mods, config.Plugins} {
		installed := server.List(kind)
		if len(installed) == 0 {
			continue
		}

		var missing []string
		for _, entry := range installed {
			if !entry.Present {
				missing = append(missing, entry.Slug)
			}
		}

		switch {
		case len(missing) > 0:
			findings = append(findings, Finding{
				Level: Problem, Title: kind.Folder(),
				Detail: fmt.Sprintf("%d of %d missing: %s — osmium install puts them back",
					len(missing), len(installed), strings.Join(missing, ", ")),
			})

		default:
			findings = append(findings, Finding{
				Title:  kind.Folder(),
				Detail: fmt.Sprintf("%d installed", len(installed)),
			})
		}

		if loose := server.Untracked(kind); len(loose) > 0 {
			findings = append(findings, Finding{
				Level: Note, Title: kind.Folder() + ", untracked",
				Detail: fmt.Sprintf("%d file(s) Osmium did not install", len(loose)),
			})
		}
	}

	return findings
}

// Worst is the most serious level in a set of findings, which is what decides
// whether a check passed.
func Worst(findings []Finding) Level {
	worst := OK
	for _, finding := range findings {
		if finding.Level > worst {
			worst = finding.Level
		}
	}
	return worst
}
