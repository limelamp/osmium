// Package cache is Osmium's download cache: the files it has fetched, kept by
// what is in them rather than by where they came from.
//
// A jar is stored under its own SHA-1, so the same file wanted by a second
// server, by a reinstall, or by a migration back to a version that was
// installed last month is already there and costs no network at all. Files are
// linked into a server rather than copied where the filesystem allows it, so
// ten servers running the same mod hold one copy of it between them.
package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Folder names inside the cache root.
const (
	filesDir = "files"
	metaDir  = "meta"
)

// Store is a cache rooted at one directory.
type Store struct {
	root string

	// off is a cache that answers nothing and keeps nothing, which is what
	// --no-cache leaves the commands holding. It is a flag rather than a nil
	// store so every caller can use one without checking.
	off bool

	// mu serialises the metadata writes, which are small and frequent enough
	// that two resolutions running at once would otherwise interleave.
	mu sync.Mutex
}

// Open returns the user's cache, creating it if this is the first run.
func Open() (*Store, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return nil, fmt.Errorf("nowhere to keep a cache: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}

	return At(filepath.Join(base, "osmium")), nil
}

// At returns a cache rooted at a given directory, which is what tests want.
func At(root string) *Store { return &Store{root: root} }

// Off returns a cache that keeps nothing, for a run asked to ignore it.
func Off() *Store { return &Store{off: true} }

// Root is where the cache lives, for a command that has to say so.
func (s *Store) Root() string { return s.root }

// Enabled reports whether anything is being kept.
func (s *Store) Enabled() bool { return s != nil && !s.off && s.root != "" }

// Path is where a file with the given SHA-1 lives, whether or not it is there
// yet. The first byte of the hash names a subdirectory, so a cache of ten
// thousand files is not ten thousand entries in one directory.
func (s *Store) Path(sha1sum string) string {
	sha1sum = strings.ToLower(sha1sum)
	if len(sha1sum) < 2 {
		return ""
	}
	return filepath.Join(s.root, filesDir, sha1sum[:2], sha1sum)
}

// Has reports whether a file is already cached.
func (s *Store) Has(sha1sum string) bool {
	if !s.Enabled() || sha1sum == "" {
		return false
	}

	info, err := os.Stat(s.Path(sha1sum))
	return err == nil && info.Size() > 0
}

// Put streams a download into the cache, checking as it goes that what
// arrived is what was promised.
//
// The file is written under a temporary name and only moved into place once
// its hash is confirmed, so an interrupted download can never be mistaken for
// a good one — the failure mode that makes a cache worse than no cache.
func (s *Store) Put(sha1sum string, body io.Reader) error {
	if !s.Enabled() {
		_, err := io.Copy(io.Discard, body)
		return err
	}

	final := s.Path(sha1sum)
	if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil {
		return fmt.Errorf("failed to make the cache folder: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(final), ".partial-*")
	if err != nil {
		return fmt.Errorf("failed to open a cache file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name()) // a no-op once the rename has happened
	}()

	digest := sha1.New() //nolint:gosec // Modrinth publishes SHA-1; it is what there is to check against
	if _, err := io.Copy(io.MultiWriter(temp, digest), body); err != nil {
		return fmt.Errorf("the download stopped: %w", err)
	}

	if got := hex.EncodeToString(digest.Sum(nil)); !strings.EqualFold(got, sha1sum) {
		return fmt.Errorf("the file that arrived is not the one that was asked for (%s, expected %s)", got, sha1sum)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("failed to finish writing the cache file: %w", err)
	}

	return os.Rename(temp.Name(), final)
}

// Link puts a cached file where a server wants it, as a hard link when the
// filesystem allows and a copy when it does not.
//
// The distinction matters at scale and never matters for correctness: a hard
// link is instant and free, and a server's jar is only ever replaced by being
// removed and written again, so the two names can never drift apart.
func (s *Store) Link(sha1sum, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}

	// A file already there is in the way of both routes.
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	source := s.Path(sha1sum)
	if s.Enabled() {
		if err := os.Link(source, dest); err == nil {
			return nil
		}
	}

	return copyFile(source, dest)
}

// Meta returns what was last stored for a request and the entity tag it came
// with, so the next request can ask whether anything has changed.
func (s *Store) Meta(key string) (body []byte, etag string) {
	if !s.Enabled() {
		return nil, ""
	}

	path := s.metaPath(key)

	body, err := os.ReadFile(path + ".json")
	if err != nil {
		return nil, ""
	}

	tag, err := os.ReadFile(path + ".etag")
	if err != nil {
		return body, ""
	}

	return body, strings.TrimSpace(string(tag))
}

// SaveMeta records an answer against its entity tag. A failure here is not
// worth reporting: the cache is an optimisation, and the caller has the answer
// it needed either way.
func (s *Store) SaveMeta(key string, body []byte, etag string) {
	if !s.Enabled() || etag == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.metaPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}

	if err := os.WriteFile(path+".json", body, 0o600); err != nil {
		return
	}
	_ = os.WriteFile(path+".etag", []byte(etag), 0o600)
}

// metaPath names the files a cached answer is kept in.
func (s *Store) metaPath(key string) string {
	sum := sha1.Sum([]byte(key)) //nolint:gosec // naming a cache entry, not securing anything
	name := hex.EncodeToString(sum[:])

	return filepath.Join(s.root, metaDir, name[:2], name)
}

// Size is how many files the cache holds and how much room they take.
func (s *Store) Size() (files int, bytes int64, err error) {
	if !s.Enabled() {
		return 0, 0, nil
	}

	err = filepath.WalkDir(s.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // an unreadable corner of the cache is not worth a failure
		}

		info, err := entry.Info()
		if err != nil {
			return nil //nolint:nilerr // likewise: the file was there a moment ago
		}

		files++
		bytes += info.Size()
		return nil
	})

	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	return files, bytes, err
}

// Clean empties the cache and reports what it reclaimed.
func (s *Store) Clean() (files int, bytes int64, err error) {
	files, bytes, err = s.Size()
	if err != nil {
		return 0, 0, err
	}

	if err := os.RemoveAll(s.root); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, 0, err
	}
	return files, bytes, nil
}

// copyFile duplicates a file, which is what a cache does when hard links are
// not available — across filesystems, or on a Windows volume that refuses.
func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// SHA1 is the hash of a file already on disk, which is how an untracked jar is
// recognised and how a cached file is checked when it is linked into a server.
func SHA1(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	digest := sha1.New() //nolint:gosec // Modrinth indexes files by SHA-1
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}
