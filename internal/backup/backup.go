// Package backup takes a copy of what a server cannot be rebuilt without.
//
// A world is the one part of a server that is nobody else's: the jar can be
// downloaded again, the mods are named in the lock, but the world exists once.
// So a backup holds the world folders and the files that describe the server,
// and leaves out everything Osmium can fetch — which is what makes it small
// enough to take before every risky thing.
package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/properties"
	"github.com/limelamp/osmium/internal/supervisor"
)

// Folder is where a server's backups are kept, beside the server itself so
// that moving the folder moves its history with it.
const Folder = ".osmium/backups"

// stamp is how a backup's name records when it was taken.
const stamp = "2006-01-02-1504"

// Backup is one snapshot.
type Backup struct {
	Name  string
	Path  string
	Size  int64
	Taken time.Time
}

// Age is how long ago it was taken, in words.
func (b Backup) Age() string {
	since := time.Since(b.Taken)

	switch {
	case since < time.Minute:
		return "just now"
	case since < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(since.Minutes()))
	case since < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(since.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(since.Hours()/24))
	}
}

// Take writes a backup of a server and reports what it wrote.
//
// The server may be running: a world saved mid-flight can be a few seconds
// behind, which for a backup is worth far more than refusing to take one.
// Stopping the server first, or running "save-off" and "save-all", is the way
// to be exact, and the caller is told so rather than prevented.
func Take(dir, label string) (Backup, error) {
	when := time.Now()

	name := when.Format(stamp)
	if label != "" {
		name += "-" + slug(label)
	}
	name += ".zip"

	folder := filepath.Join(dir, Folder)
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return Backup{}, err
	}

	path := filepath.Join(folder, name)

	file, err := os.Create(path) //nolint:gosec // a path built from the server's own folder
	if err != nil {
		return Backup{}, err
	}

	archive := zip.NewWriter(file)

	for _, item := range contents(dir) {
		if err := add(archive, dir, item); err != nil {
			_ = archive.Close()
			_ = file.Close()
			_ = os.Remove(path)

			return Backup{}, fmt.Errorf("failed to back up %s: %w", item, err)
		}
	}

	if err := archive.Close(); err != nil {
		_ = file.Close()
		return Backup{}, err
	}
	if err := file.Close(); err != nil {
		return Backup{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return Backup{}, err
	}

	return Backup{Name: name, Path: path, Size: info.Size(), Taken: when}, nil
}

// contents is what a backup holds: the worlds, and the files that say what the
// server is and who is allowed on it.
func contents(dir string) []string {
	var items []string

	world := "world"
	if props, err := properties.Read(dir); err == nil {
		if name := props.Get(properties.KeyLevelName); name != "" {
			world = name
		}
	}

	for _, suffix := range []string{"", "_nether", "_the_end"} {
		candidate := world + suffix
		if info, err := os.Stat(filepath.Join(dir, candidate)); err == nil && info.IsDir() {
			items = append(items, candidate)
		}
	}

	for _, file := range []string{
		config.OsmiumFileName, config.LockFileName, properties.FileName,
		"ops.json", "whitelist.json", "banned-players.json", "banned-ips.json",
		"eula.txt", "server-icon.png",
	} {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			items = append(items, file)
		}
	}

	return items
}

// add puts one file or folder into the archive.
func add(archive *zip.Writer, root, item string) error {
	source := filepath.Join(root, item)

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		writer, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}

		file, err := os.Open(path) //nolint:gosec // walking the server's own folder
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(writer, file)
		return err
	})
}

// List is a server's backups, newest first.
func List(dir string) []Backup {
	entries, err := os.ReadDir(filepath.Join(dir, Folder))
	if err != nil {
		return nil
	}

	backups := make([]Backup, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		taken := info.ModTime()
		if parsed, err := time.Parse(stamp, entry.Name()[:min(len(entry.Name()), len(stamp))]); err == nil {
			taken = parsed
		}

		backups = append(backups, Backup{
			Name:  entry.Name(),
			Path:  filepath.Join(dir, Folder, entry.Name()),
			Size:  info.Size(),
			Taken: taken,
		})
	}

	sort.Slice(backups, func(a, b int) bool { return backups[a].Taken.After(backups[b].Taken) })
	return backups
}

// Restore puts a backup back over a server.
//
// The server has to be stopped: writing a world out from under a running
// server corrupts it, and there is no version of that which ends well.
func Restore(dir string, chosen Backup) error {
	if supervisor.Running(dir) {
		return fmt.Errorf("stop the server first — restoring under a running server would corrupt the world")
	}

	archive, err := zip.OpenReader(chosen.Path)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()

	for _, entry := range archive.File {
		dest := filepath.Join(dir, filepath.FromSlash(entry.Name))

		// An archive is a file like any other; a path that climbs out of the
		// server folder is refused rather than followed.
		if !strings.HasPrefix(dest, filepath.Clean(dir)+string(os.PathSeparator)) {
			continue
		}

		if entry.FileInfo().IsDir() {
			continue
		}

		if err := write(entry, dest); err != nil {
			return err
		}
	}

	return nil
}

// write puts one archived file back on disk.
func write(entry *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}

	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, source); err != nil { //nolint:gosec // an archive this program wrote
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Remove deletes one backup.
func Remove(chosen Backup) error { return os.Remove(chosen.Path) }

// slug makes a label safe for a file name.
func slug(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))

	kept := make([]rune, 0, len(label))
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			kept = append(kept, r)
		case r == ' ', r == '-', r == '_':
			kept = append(kept, '-')
		}
	}

	return strings.Trim(string(kept), "-")
}
