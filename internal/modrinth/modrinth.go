// Package modrinth talks to the Modrinth API: what a project is, which of its
// builds fit a server, and what each build needs.
//
// Everything here is read-only and cacheable, which is what the package is
// built around. One project asked for four times in a resolution is fetched
// once; a hundred files identified by hash go out as one request rather than a
// hundred; and metadata that has not changed since it was last seen comes back
// as a 304 with no body to parse. That is the difference between a resolver
// that feels instant and one that spends a second per mod.
package modrinth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// API is the base of every request. v2 is the stable one; v3 is still marked
// as subject to change.
const API = "https://api.modrinth.com/v2"

// userAgent identifies Osmium to Modrinth, which asks that tools say who they
// are so it can tell a well-behaved client from a runaway script.
const userAgent = "limelamp/osmium (github.com/limelamp/osmium)"

// batchLimit is how many ids go into one request. Modrinth accepts long query
// strings, but a URL that grows without limit eventually meets a proxy that
// disagrees, so the ids are sent in chunks.
const batchLimit = 100

// ErrNotFound is returned for a project or version Modrinth does not have.
var ErrNotFound = errors.New("not found on Modrinth")

// Dependency types, as Modrinth names them.
const (
	Required     = "required"
	Optional     = "optional"
	Incompatible = "incompatible"
	Embedded     = "embedded"
)

// Release channels, in the order a resolver prefers them.
const (
	Release = "release"
	Beta    = "beta"
	Alpha   = "alpha"
)

// Project is a mod or a plugin as Modrinth describes it.
type Project struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	ProjectType string   `json:"project_type"`
	Loaders     []string `json:"loaders"`
	ServerSide  string   `json:"server_side"`
}

// File is one downloadable artefact of a version.
type File struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Size     int64  `json:"size"`
	Hashes   struct {
		SHA1   string `json:"sha1"`
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

// Dependency is one thing a version needs, or refuses to sit beside.
type Dependency struct {
	ProjectID string `json:"project_id"`
	VersionID string `json:"version_id"`
	Type      string `json:"dependency_type"`
}

// Version is one build of a project.
type Version struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Name          string       `json:"name"`
	VersionNumber string       `json:"version_number"`
	GameVersions  []string     `json:"game_versions"`
	Loaders       []string     `json:"loaders"`
	VersionType   string       `json:"version_type"`
	DatePublished time.Time    `json:"date_published"`
	Files         []File       `json:"files"`
	Dependencies  []Dependency `json:"dependencies"`
}

// Primary is the file to install: the one Modrinth marks primary, or the first
// one when a version marks none.
func (v Version) Primary() (File, bool) {
	for _, file := range v.Files {
		if file.Primary {
			return file, true
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], true
	}
	return File{}, false
}

// Stable reports whether this is a full release rather than a beta or alpha.
func (v Version) Stable() bool { return v.VersionType == Release }

// Cache is where answers are kept between runs, so metadata that has not
// changed is not fetched or parsed twice. The store in internal/cache is what
// fills it; a nil one leaves the client remembering only within its own run.
type Cache interface {
	// Meta returns what was last stored for a request, and the entity tag the
	// server gave it.
	Meta(key string) (body []byte, etag string)

	// SaveMeta records an answer against its entity tag.
	SaveMeta(key string, body []byte, etag string)
}

// Client is a Modrinth session: one connection pool, one rate limit, and one
// cache of what has already been asked.
type Client struct {
	http  *http.Client
	store Cache

	// base is where the API lives. It is a field rather than the constant so
	// that a test can answer for Modrinth without the network.
	base string

	// mu guards the in-memory results, which are what stops a resolution
	// asking for the same project once per mod that depends on it.
	mu       sync.Mutex
	projects map[string]Project
	versions map[string][]Version

	// pause is when the next request may go out. Modrinth answers a request
	// too many with 429 and a Retry-After, and the polite thing — the thing
	// that stops the next twenty requests failing too — is to wait it out.
	pause time.Time
}

// New returns a client that caches metadata in the given store. A nil store
// means memory only, which is what tests want.
func New(store Cache) *Client { return NewAt(API, store) }

// NewAt is New pointed at a different base URL, which is what tests use.
func NewAt(base string, store Cache) *Client {
	return &Client{
		base: base,
		http: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		store:    store,
		projects: make(map[string]Project),
		versions: make(map[string][]Version),
	}
}

// Project looks a project up by id or slug.
func (c *Client) Project(ctx context.Context, id string) (Project, error) {
	c.mu.Lock()
	if project, seen := c.projects[strings.ToLower(id)]; seen {
		c.mu.Unlock()
		return project, nil
	}
	c.mu.Unlock()

	var project Project
	if err := c.get(ctx, c.base+"/project/"+url.PathEscape(id), &project); err != nil {
		return Project{}, fmt.Errorf("%s: %w", id, err)
	}

	c.remember(project)
	return project, nil
}

// Projects looks up many projects at once, which is one request instead of
// one per project. Anything already known is answered from memory.
func (c *Client) Projects(ctx context.Context, ids []string) (map[string]Project, error) {
	found := make(map[string]Project, len(ids))

	var missing []string
	c.mu.Lock()
	for _, id := range ids {
		if project, seen := c.projects[strings.ToLower(id)]; seen {
			found[strings.ToLower(id)] = project
			continue
		}
		missing = append(missing, id)
	}
	c.mu.Unlock()

	for chunk := range batches(missing) {
		var projects []Project
		if err := c.get(ctx, c.base+"/projects?ids="+encodeIDs(chunk), &projects); err != nil {
			return nil, err
		}

		for _, project := range projects {
			c.remember(project)
			found[strings.ToLower(project.ID)] = project
			found[strings.ToLower(project.Slug)] = project
		}
	}

	return found, nil
}

// Versions is every build of a project that fits the given loaders and
// Minecraft version, newest first.
//
// The filtering is done by Modrinth rather than here: asking for the builds
// that fit is one small answer, where asking for all of them and sifting can
// be a megabyte for a mod with a long history.
func (c *Client) Versions(ctx context.Context, project string, loaders []string, game string) ([]Version, error) {
	key := strings.ToLower(project) + "|" + strings.Join(loaders, ",") + "|" + game

	c.mu.Lock()
	if versions, seen := c.versions[key]; seen {
		c.mu.Unlock()
		return versions, nil
	}
	c.mu.Unlock()

	endpoint := fmt.Sprintf("%s/project/%s/version?loaders=%s&game_versions=%s",
		c.base, url.PathEscape(project), encodeStrings(loaders), encodeStrings([]string{game}))

	var versions []Version
	if err := c.get(ctx, endpoint, &versions); err != nil {
		return nil, fmt.Errorf("%s: %w", project, err)
	}

	// Newest first, so "the newest that fits" is a walk from the front.
	sortByDate(versions)

	c.mu.Lock()
	c.versions[key] = versions
	c.mu.Unlock()

	return versions, nil
}

// Version reads one build by its id, which is what a pin resolves to.
func (c *Client) Version(ctx context.Context, id string) (Version, error) {
	var version Version
	if err := c.get(ctx, c.base+"/version/"+url.PathEscape(id), &version); err != nil {
		return Version{}, fmt.Errorf("version %s: %w", id, err)
	}
	return version, nil
}

// ByHashes identifies files by their SHA-1, in one request for all of them.
// It is how a folder of jars nobody has recorded becomes a tracked set.
func (c *Client) ByHashes(ctx context.Context, hashes []string) (map[string]Version, error) {
	found := make(map[string]Version, len(hashes))

	for chunk := range batches(hashes) {
		body, err := json.Marshal(map[string]any{"hashes": chunk, "algorithm": "sha1"})
		if err != nil {
			return nil, err
		}

		var versions map[string]Version
		if err := c.post(ctx, c.base+"/version_files", body, &versions); err != nil {
			return nil, err
		}

		for hash, version := range versions {
			found[strings.ToLower(hash)] = version
		}
	}

	return found, nil
}

// Search finds projects by name, which is how a mod is added by something
// other than its exact slug.
func (c *Client) Search(ctx context.Context, query string, loaders []string, game string, limit int) ([]Project, error) {
	facets := [][]string{{"project_type:mod", "project_type:plugin", "project_type:modpack"}}
	if game != "" {
		facets = append(facets, []string{"versions:" + game})
	}
	if len(loaders) > 0 {
		loaderFacet := make([]string, 0, len(loaders))
		for _, loader := range loaders {
			loaderFacet = append(loaderFacet, "categories:"+loader)
		}
		facets = append(facets, loaderFacet)
	}

	encoded, err := json.Marshal(facets)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/search?query=%s&facets=%s&limit=%d",
		c.base, url.QueryEscape(query), url.QueryEscape(string(encoded)), limit)

	var answer struct {
		Hits []struct {
			ProjectID   string   `json:"project_id"`
			Slug        string   `json:"slug"`
			Title       string   `json:"title"`
			ProjectType string   `json:"project_type"`
			Categories  []string `json:"categories"`
		} `json:"hits"`
	}
	if err := c.get(ctx, endpoint, &answer); err != nil {
		return nil, err
	}

	projects := make([]Project, 0, len(answer.Hits))
	for _, hit := range answer.Hits {
		projects = append(projects, Project{
			ID: hit.ProjectID, Slug: hit.Slug, Title: hit.Title,
			ProjectType: hit.ProjectType, Loaders: hit.Categories,
		})
	}
	return projects, nil
}

// remember keeps a project under both the names it answers to, so a
// dependency given by id and a request given by slug hit the same entry.
func (c *Client) remember(project Project) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.projects[strings.ToLower(project.ID)] = project
	c.projects[strings.ToLower(project.Slug)] = project
}

func (c *Client) get(ctx context.Context, endpoint string, into any) error {
	return c.do(ctx, http.MethodGet, endpoint, nil, into)
}

func (c *Client) post(ctx context.Context, endpoint string, body []byte, into any) error {
	return c.do(ctx, http.MethodPost, endpoint, body, into)
}

// do performs a request, waiting out a rate limit, retrying what is worth
// retrying, and answering from the disk cache when the server says the copy
// there is still good.
func (c *Client) do(ctx context.Context, method, endpoint string, body []byte, into any) error {
	// Only plain reads are worth caching; a hash lookup posts its question.
	cacheable := method == http.MethodGet && c.store != nil

	var cached []byte
	var etag string
	if cacheable {
		cached, etag = c.store.Meta(endpoint)
	}

	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
		}

		c.waitForRateLimit(ctx)

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}

		resp, err := c.http.Do(req) //nolint:bodyclose // readAll below drains and closes it
		if err != nil {
			lastErr = err
			continue
		}

		payload, status, header, err := readAll(resp)
		if err != nil {
			lastErr = err
			continue
		}

		switch {
		case status == http.StatusNotModified && len(cached) > 0:
			return json.Unmarshal(cached, into)

		case status == http.StatusOK:
			if cacheable {
				c.store.SaveMeta(endpoint, payload, header.Get("ETag"))
			}
			return json.Unmarshal(payload, into)

		case status == http.StatusNotFound || status == http.StatusGone:
			return ErrNotFound

		case status == http.StatusTooManyRequests:
			c.holdOff(header.Get("Retry-After"))
			lastErr = fmt.Errorf("modrinth is rate limiting: %s", resp.Status)

		case status >= 500:
			lastErr = fmt.Errorf("modrinth is unwell: %s", resp.Status)

		default:
			return fmt.Errorf("modrinth answered %s", resp.Status)
		}
	}

	// The cache is better than nothing when the network is not answering.
	if len(cached) > 0 {
		return json.Unmarshal(cached, into)
	}
	return lastErr
}

// waitForRateLimit blocks until the moment a 429 told us to come back.
func (c *Client) waitForRateLimit(ctx context.Context) {
	c.mu.Lock()
	pause := c.pause
	c.mu.Unlock()

	if wait := time.Until(pause); wait > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

// holdOff records when the next request may go out.
func (c *Client) holdOff(retryAfter string) {
	wait := 2 * time.Second
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		wait = min(time.Duration(seconds)*time.Second, time.Minute)
	}

	c.mu.Lock()
	c.pause = time.Now().Add(wait)
	c.mu.Unlock()
}

// readAll drains and closes a response, so the connection goes back to the
// pool rather than being abandoned mid-body.
func readAll(resp *http.Response) (body []byte, status int, header http.Header, err error) {
	defer func() { _ = resp.Body.Close() }()

	body, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return body, resp.StatusCode, resp.Header, err
}

// batches cuts a list into request-sized pieces.
func batches(ids []string) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for start := 0; start < len(ids); start += batchLimit {
			if !yield(ids[start:min(start+batchLimit, len(ids))]) {
				return
			}
		}
	}
}

// encodeIDs renders a list of ids the way Modrinth's batch endpoints want it.
func encodeIDs(ids []string) string { return encodeStrings(ids) }

// encodeStrings renders a JSON array of strings into a query parameter.
func encodeStrings(values []string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return url.QueryEscape(string(encoded))
}

// sortByDate puts the newest build first.
func sortByDate(versions []Version) {
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && versions[j].DatePublished.After(versions[j-1].DatePublished); j-- {
			versions[j], versions[j-1] = versions[j-1], versions[j]
		}
	}
}
