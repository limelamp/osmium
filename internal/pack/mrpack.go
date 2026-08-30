package pack

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// indexName is the manifest inside a Modrinth modpack.
const indexName = "modrinth.index.json"

// Modpack is what a .mrpack file describes: a Minecraft version, a loader, and
// the files that make up the pack.
//
// It is read rather than written. A modpack is somebody else's recipe, and the
// point of understanding it is that "install this modpack" becomes the same
// command as everything else instead of a separate ceremony.
type Modpack struct {
	Name       string
	Version    string
	Game       string
	Loader     string
	LoaderVer  string
	Files      []ModpackFile
	Overrides  []string
	sourcePath string
}

// ModpackFile is one file the pack wants installed.
type ModpackFile struct {
	Path      string
	Downloads []string
	SHA1      string
	SHA512    string
	Size      int64

	// Server says whether the file belongs on a server at all: a pack lists
	// client-only mods too, and installing those is how a server ends up
	// refusing to start.
	Server bool
}

// index is the modpack manifest as it is written.
type index struct {
	FormatVersion int    `json:"formatVersion"`
	Name          string `json:"name"`
	VersionID     string `json:"versionId"`
	Files         []struct {
		Path      string            `json:"path"`
		Hashes    map[string]string `json:"hashes"`
		Downloads []string          `json:"downloads"`
		FileSize  int64             `json:"fileSize"`
		Env       struct {
			Client string `json:"client"`
			Server string `json:"server"`
		} `json:"env"`
	} `json:"files"`
	Dependencies map[string]string `json:"dependencies"`
}

// ReadModpack opens a .mrpack and reads what it asks for.
func ReadModpack(path string) (*Modpack, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("%s is not a readable modpack: %w", filepath.Base(path), err)
	}
	defer func() { _ = archive.Close() }()

	var manifest *index
	var overrides []string

	for _, entry := range archive.File {
		switch {
		case entry.Name == indexName:
			manifest, err = readIndex(entry)
			if err != nil {
				return nil, err
			}

		case strings.HasPrefix(entry.Name, "overrides/"),
			strings.HasPrefix(entry.Name, "server-overrides/"):
			if !entry.FileInfo().IsDir() {
				overrides = append(overrides, entry.Name)
			}
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("%s has no %s in it", filepath.Base(path), indexName)
	}

	packed := &Modpack{
		Name:       manifest.Name,
		Version:    manifest.VersionID,
		Game:       manifest.Dependencies["minecraft"],
		Overrides:  overrides,
		sourcePath: path,
	}

	packed.Loader, packed.LoaderVer = loaderOf(manifest.Dependencies)

	for _, file := range manifest.Files {
		// A pack marks each file for the client, the server, or both. Only
		// what a server is meant to load is installed.
		if file.Env.Server == "unsupported" {
			continue
		}

		packed.Files = append(packed.Files, ModpackFile{
			Path:      filepath.FromSlash(file.Path),
			Downloads: file.Downloads,
			SHA1:      file.Hashes["sha1"],
			SHA512:    file.Hashes["sha512"],
			Size:      file.FileSize,
			Server:    true,
		})
	}

	return packed, nil
}

// readIndex parses the manifest out of the archive.
func readIndex(entry *zip.File) (*index, error) {
	file, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		return nil, err
	}

	var manifest index
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("the modpack's index is not readable: %w", err)
	}
	return &manifest, nil
}

// loaderOf works out which loader a pack wants from its dependencies, which
// name the loader by its own key.
func loaderOf(dependencies map[string]string) (loader, version string) {
	for key, value := range dependencies {
		switch key {
		case "fabric-loader":
			return "fabric", value
		case "quilt-loader":
			return "quilt", value
		case "forge":
			return "forge", value
		case "neoforge":
			return "neoforge", value
		}
	}
	return "", ""
}

// ExtractOverrides copies the pack's own files — configs, scripts, a world —
// into a server directory. They are written before anything is downloaded, so
// a pack's configuration is in place when its mods first load.
func (m *Modpack) ExtractOverrides(dir string) (int, error) {
	if len(m.Overrides) == 0 {
		return 0, nil
	}

	archive, err := zip.OpenReader(m.sourcePath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = archive.Close() }()

	written := 0
	for _, entry := range archive.File {
		name := entry.Name

		var relative string
		switch {
		case strings.HasPrefix(name, "server-overrides/"):
			relative = strings.TrimPrefix(name, "server-overrides/")
		case strings.HasPrefix(name, "overrides/"):
			relative = strings.TrimPrefix(name, "overrides/")
		default:
			continue
		}

		if relative == "" || entry.FileInfo().IsDir() {
			continue
		}

		// A path out of the archive is a path out of the server directory; a
		// modpack is a file from the internet and is treated as one.
		dest := filepath.Join(dir, filepath.FromSlash(relative))
		if !strings.HasPrefix(dest, filepath.Clean(dir)+string(os.PathSeparator)) {
			continue
		}

		if err := writeEntry(entry, dest); err != nil {
			return written, err
		}
		written++
	}

	return written, nil
}

// writeEntry puts one file from the archive on disk.
func writeEntry(entry *zip.File, dest string) error {
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

	// A modpack is untrusted input, so the copy is bounded rather than open
	// ended: a zip that claims to hold a terabyte does not get to write one.
	if _, err := io.Copy(out, io.LimitReader(source, 1<<30)); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// InstallInto puts a modpack's files into a server directory: its own files
// first, then everything it lists, downloaded in parallel and checked against
// the hashes the pack gives.
//
// What lands in mods/ is identified afterwards by Track, so a pack installed
// this way ends up as a properly locked set rather than a folder of jars
// nobody can account for.
func (m *Modpack) InstallInto(ctx context.Context, dir string, opts Options) (int, error) {
	store, done, err := staging(opts.Cache)
	if err != nil {
		return 0, err
	}
	defer done()

	items := make([]wanted, 0, len(m.Files))
	for _, file := range m.Files {
		if file.SHA1 == "" || len(file.Downloads) == 0 {
			continue
		}

		items = append(items, wanted{
			name: filepath.Base(file.Path),
			urls: file.Downloads,
			sha1: file.SHA1,
			size: file.Size,
		})
	}

	if err := fetchAll(ctx, items, store, opts); err != nil {
		return 0, err
	}

	installed := 0
	for _, file := range m.Files {
		if file.SHA1 == "" || len(file.Downloads) == 0 {
			continue
		}

		dest := filepath.Join(dir, file.Path)
		if !strings.HasPrefix(dest, filepath.Clean(dir)+string(os.PathSeparator)) {
			continue
		}

		if err := store.Link(file.SHA1, dest); err != nil {
			return installed, fmt.Errorf("failed to install %s: %w", file.Path, err)
		}
		installed++
	}

	return installed, nil
}
