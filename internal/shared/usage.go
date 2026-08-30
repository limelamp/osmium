package shared

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Usage is how hard a server's process is working.
type Usage struct {
	// Memory is the resident set: what the process actually holds, which is
	// the number worth showing. The heap size the JVM was given is what it may
	// take, not what it has.
	Memory int64

	// CPU is processor time used since the process started. A percentage is a
	// difference between two of these, which is why the sample carries the
	// total and the moment it was taken rather than a rate.
	CPU  time.Duration
	When time.Time

	Known bool
}

// Percent is the share of one processor used between two samples.
func (u Usage) Percent(previous Usage) float64 {
	if !u.Known || !previous.Known {
		return 0
	}

	elapsed := u.When.Sub(previous.When)
	if elapsed <= 0 {
		return 0
	}

	used := u.CPU - previous.CPU
	if used < 0 {
		return 0
	}

	return float64(used) / float64(elapsed) * 100
}

// Sample reads what a process is using, without disturbing it.
func Sample(pid int) Usage {
	if pid <= 0 {
		return Usage{}
	}

	switch runtime.GOOS {
	case "linux":
		return sampleLinux(pid)
	case "darwin":
		return sampleDarwin(pid)
	default:
		return Usage{}
	}
}

// sampleLinux reads /proc, which is what every tool on Linux does and costs
// nothing but a file read.
func sampleLinux(pid int) Usage {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return Usage{}
	}

	// The process name can contain spaces and brackets, so the fields after it
	// are counted from the last closing bracket rather than from the start.
	line := string(data)
	end := strings.LastIndex(line, ")")
	if end < 0 {
		return Usage{}
	}

	fields := strings.Fields(line[end+1:])
	if len(fields) < 22 {
		return Usage{}
	}

	// Fields, counting the one after the name as the first: utime is 12,
	// stime 13, and rss (in pages) 22.
	utime, _ := strconv.ParseInt(fields[11], 10, 64)
	stime, _ := strconv.ParseInt(fields[12], 10, 64)
	rss, _ := strconv.ParseInt(fields[21], 10, 64)

	const ticksPerSecond = 100 // the kernel's USER_HZ, fixed on every Linux worth running a server on

	return Usage{
		Memory: rss * int64(os.Getpagesize()),
		CPU:    time.Duration(utime+stime) * time.Second / ticksPerSecond,
		When:   time.Now(),
		Known:  true,
	}
}

// sampleDarwin asks ps, there being no /proc to read.
func sampleDarwin(pid int) Usage {
	out, err := exec.Command("ps", "-o", "rss=,time=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec,noctx // a fixed command with a number
	if err != nil {
		return Usage{}
	}

	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return Usage{}
	}

	kb, _ := strconv.ParseInt(fields[0], 10, 64)

	return Usage{
		Memory: kb * 1024,
		CPU:    parseClock(fields[1]),
		When:   time.Now(),
		Known:  true,
	}
}

// parseClock reads the "mm:ss.hh" ps writes processor time as.
func parseClock(value string) time.Duration {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}

	minutes, _ := strconv.Atoi(parts[0])
	seconds, _ := strconv.ParseFloat(parts[1], 64)

	return time.Duration(minutes)*time.Minute + time.Duration(seconds*float64(time.Second))
}
