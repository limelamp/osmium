package pack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
)

// workers is how many files are fetched at once.
//
// Four is chosen against Modrinth rather than against the network: it saturates
// a home connection on files this size while staying far inside what the API
// asks of a client, and it keeps the progress report meaningful instead of
// turning it into twenty bars fighting for one line.
const workers = 4

// undoDir is where files that are about to be replaced or removed are kept
// until the rest of the plan has gone through.
const undoDir = ".osmium/undo"

// Options are what applying a plan needs from the outside: where to keep
// downloads, and where to report what is happening.
type Options struct {
	Cache *cache.Store
	Scope scope.Scope

	// HTTP is the client downloads go through. A zero value uses the default,
	// which is what everything but a test wants.
	HTTP *http.Client
}

// Apply carries a plan out.
//
// Every file is fetched and checked before anything in the server is touched,
// and everything the plan replaces or removes is set aside rather than deleted
// until the whole plan has gone through. A failure at any point therefore
// leaves the server exactly as it was — the property that makes it safe to run
// an upgrade on a server you care about.
func (s *Server) Apply(ctx context.Context, plan Plan, opts Options) error {
	if plan.Blocked() {
		return fmt.Errorf("the plan cannot be applied: %s", plan.Conflicts[0])
	}
	if plan.Empty() {
		s.record(plan)
		return s.Save()
	}

	store, done, err := staging(opts.Cache)
	if err != nil {
		return err
	}
	defer done()

	if err := fetchAll(ctx, plan.downloads(), store, opts); err != nil {
		return err
	}

	if err := s.move(plan, store, opts.Scope); err != nil {
		return err
	}

	s.record(plan)
	return s.Save()
}

// staging is where downloads land before they are checked and moved in.
//
// It is the cache when there is one. A run told to ignore the cache still
// needs somewhere to put a file while its hash is being confirmed, and a
// temporary directory keeps every download on the same one path.
func staging(store *cache.Store) (*cache.Store, func(), error) {
	if store.Enabled() {
		return store, func() {}, nil
	}

	temp, err := os.MkdirTemp("", "osmium-download-")
	if err != nil {
		return nil, nil, err
	}

	return cache.At(temp), func() { _ = os.RemoveAll(temp) }, nil
}

// wanted is one file to fetch: where it is, what it should hash to, and what
// to call it in the log.
type wanted struct {
	name string
	urls []string
	sha1 string
	size int64
}

// fetchAll downloads what is not already cached, several at a time.
//
// The progress reported is the total across all of them rather than one bar
// per file: four bars racing each other on one line says less than one bar
// that means "this operation is two thirds done".
func fetchAll(ctx context.Context, items []wanted, store *cache.Store, opts Options) error {
	var pending []wanted
	var total int64

	for _, item := range items {
		if store.Has(item.sha1) {
			continue
		}
		pending = append(pending, item)
		total += item.size
	}

	if len(pending) == 0 {
		return nil
	}

	label := fmt.Sprintf("downloading %d files", len(pending))
	if len(pending) == 1 {
		label = "downloading " + pending[0].name
	}
	opts.Scope.Report(label, 0, total)

	var done atomic.Int64
	var once sync.Once
	var failure error

	queue := make(chan wanted)
	var wait sync.WaitGroup

	for range min(workers, len(pending)) {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for item := range queue {
				err := download(ctx, item, store, opts, func(n int64) {
					opts.Scope.Report(label, done.Add(n), total)
				})
				if err != nil {
					once.Do(func() { failure = err })
					return
				}
				opts.Scope.Logf("fetched %s", item.name)
			}
		}()
	}

	for _, item := range pending {
		select {
		case queue <- item:
		case <-ctx.Done():
			failure = ctx.Err()
		}
		if failure != nil {
			break
		}
	}

	close(queue)
	wait.Wait()

	// The bar is finished either way; a failed plan should not leave one
	// hanging at seventy per cent.
	opts.Scope.Report(label, total, total)

	return failure
}

// download fetches one file into the cache, where its hash is checked.
//
// A file may be published in more than one place; each is tried in turn, so a
// mirror being down is not an install being down.
func download(ctx context.Context, item wanted, store *cache.Store, opts Options, progress func(int64)) error {
	client := opts.HTTP
	if client == nil {
		client = http.DefaultClient
	}

	var last error
	for _, from := range item.urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, from, http.NoBody)
		if err != nil {
			last = err
			continue
		}
		req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

		resp, err := client.Do(req)
		if err != nil {
			last = fmt.Errorf("%s: %w", item.name, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			last = fmt.Errorf("%s: the download answered %s", item.name, resp.Status)
			continue
		}

		body := io.Reader(resp.Body)
		if progress != nil {
			body = io.TeeReader(resp.Body, counter(progress))
		}

		err = store.Put(item.sha1, body)
		_ = resp.Body.Close()

		if err != nil {
			last = fmt.Errorf("%s: %w", item.name, err)
			continue
		}
		return nil
	}

	if last == nil {
		last = fmt.Errorf("%s has nowhere to download from", item.name)
	}
	return last
}

// move puts the downloaded files into the server and takes the old ones out,
// undoing the lot if any step of it fails.
func (s *Server) move(plan Plan, store *cache.Store, sc scope.Scope) error {
	undo := filepath.Join(s.Dir, undoDir)
	if err := os.MkdirAll(undo, 0o750); err != nil {
		return fmt.Errorf("failed to make room to work in: %w", err)
	}

	// steps remembers what to put back, in reverse, if this goes wrong.
	var steps []step

	rollback := func() {
		for i := len(steps) - 1; i >= 0; i-- {
			steps[i].undo()
		}
		_ = os.RemoveAll(undo)
	}

	for _, change := range plan.Changes {
		folder := s.Folder(change.Kind)

		// Whatever is there now is set aside, not deleted.
		if old, present := s.replacing(change); present {
			kept := filepath.Join(undo, filepath.Base(old))
			if err := os.Rename(old, kept); err != nil {
				rollback()
				return fmt.Errorf("failed to set %s aside: %w", filepath.Base(old), err)
			}

			steps = append(steps, step{undo: func() { _ = os.Rename(kept, old) }})
		}

		if change.Action == Remove {
			sc.Logf("removed %s %s", change.Slug, change.From)
			continue
		}

		dest := filepath.Join(folder, change.Entry.FileName)
		if err := store.Link(change.Entry.SHA1, dest); err != nil {
			rollback()
			return fmt.Errorf("failed to install %s: %w", change.Slug, err)
		}

		steps = append(steps, step{undo: func() { _ = os.Remove(dest) }})
		sc.Logf("%s %s %s", change.Action.Verb()+"ed", change.Slug, change.To)
	}

	return os.RemoveAll(undo)
}

// replacing is the file a change is about to replace, if there is one.
func (s *Server) replacing(change Change) (string, bool) {
	if change.Entry.FileName == "" && change.From == "" {
		return "", false
	}

	// A removal and an upgrade both have an old file named by the lock.
	existing, locked := s.Lock.Find(change.Kind, change.Slug)
	if !locked {
		return "", false
	}

	return s.Installed(existing)
}

// record applies the plan to the manifest and the lock in memory, ready to be
// written.
func (s *Server) record(plan Plan) {
	for _, change := range plan.Changes {
		if change.Action == Remove {
			s.Lock.Remove(change.Kind, change.Slug)
			continue
		}
		s.Lock.Put(change.Entry)
	}

	if plan.wants != nil {
		if plan.wants.Kind == config.Plugins {
			s.Conf.Plugins = plan.wants.Wants
		} else {
			s.Conf.Mods = plan.wants.Wants
		}
	}

	s.Lock.Loader, s.Lock.Game = s.Conf.Loader, s.Conf.Version
}

// step is one thing done to the server folder, and how to take it back.
type step struct{ undo func() }

// counter is a writer that reports how much has gone past.
type counter func(int64)

func (c counter) Write(p []byte) (int, error) {
	c(int64(len(p)))
	return len(p), nil
}

// ErrCancelled is what a plan the user backed out of returns.
var ErrCancelled = errors.New("cancelled")
