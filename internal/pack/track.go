package pack

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
)

// Adopted is one file that was in a server's folder and is now accounted for.
type Adopted struct {
	FileName string
	Slug     string
	Version  string
	Known    bool
}

// Track takes the jars a server already has and works out what they are, so a
// folder somebody has been filling by hand becomes a managed set.
//
// Every file is identified by its hash in a single request rather than one
// each: a hundred jars is one round trip, which is the difference between
// tracking a big modpack in a second and doing it in a minute.
func (r *Resolver) Track(ctx context.Context, kind config.Kind) ([]Adopted, error) {
	loose := r.Server.Untracked(kind)
	if len(loose) == 0 {
		return nil, nil
	}

	hashes := make([]string, 0, len(loose))
	byHash := make(map[string]string, len(loose))

	for _, name := range loose {
		sum, err := cache.SHA1(filepath.Join(r.Server.Folder(kind), name))
		if err != nil {
			continue
		}

		hashes = append(hashes, sum)
		byHash[sum] = name
	}

	if len(hashes) == 0 {
		return nil, nil
	}

	versions, err := r.Client.ByHashes(ctx, hashes)
	if err != nil {
		return nil, err
	}

	// The projects behind the versions, in one request as well.
	ids := make([]string, 0, len(versions))
	for _, version := range versions {
		ids = append(ids, version.ProjectID)
	}

	projects, err := r.Client.Projects(ctx, ids)
	if err != nil {
		projects = nil
	}

	adopted := make([]Adopted, 0, len(loose))
	for _, sum := range hashes {
		name := byHash[sum]

		version, known := versions[sum]
		if !known {
			adopted = append(adopted, Adopted{FileName: name})
			continue
		}

		slug := version.ProjectID
		if project, ok := projects[version.ProjectID]; ok {
			slug = project.Slug
		}

		entry := config.Locked{
			Slug:          slug,
			Kind:          kind,
			ProjectID:     version.ProjectID,
			VersionID:     version.ID,
			VersionNumber: version.VersionNumber,
			VersionType:   version.VersionType,
			FileName:      name,
			Explicit:      true,
		}

		if file, ok := version.Primary(); ok {
			entry.DownloadURL = file.URL
			entry.SHA1 = file.Hashes.SHA1
			entry.Size = file.Size
		}

		// The file on disk is the truth about what is installed, whatever
		// Modrinth calls the primary file of that build.
		entry.FileName = name
		entry.SHA1 = sum

		r.Server.Lock.Put(entry)
		r.Server.Conf.Want(kind)[slug] = config.Newest

		adopted = append(adopted, Adopted{
			FileName: name, Slug: slug, Version: version.VersionNumber, Known: true,
		})
	}

	sort.Slice(adopted, func(a, b int) bool { return adopted[a].FileName < adopted[b].FileName })
	return adopted, r.Server.Save()
}

// Line describes one adopted file.
func (a Adopted) Line() string {
	if !a.Known {
		return fmt.Sprintf("unknown  %s   not on Modrinth, left alone", a.FileName)
	}
	return fmt.Sprintf("tracked  %s %s", a.Slug, a.Version)
}

// Installed lists what a server has, newest information first: what the lock
// says, and whether the file is actually where it should be.
type Installed struct {
	config.Locked
	Present bool
}

// List is everything installed of one kind.
func (s *Server) List(kind config.Kind) []Installed {
	entries := s.Lock.Of(kind)

	list := make([]Installed, 0, len(entries))
	for _, entry := range entries {
		_, present := s.Installed(entry)
		list = append(list, Installed{Locked: entry, Present: present})
	}

	return list
}

// VersionChoice is one build of a project this server could install. The list
// of them is what "osmium mod versions" shows and what a downgrade is chosen
// from.
type VersionChoice struct {
	VersionNumber string
	VersionID     string
	Channel       string
	Installed     bool
	Size          int64
}

// Versions lists the builds available to this server for one project.
func (r *Resolver) Versions(ctx context.Context, kind config.Kind, slug string) ([]VersionChoice, error) {
	key := r.resolveSlug(kind, slug)

	project, err := r.Client.Project(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", slug, err)
	}

	versions, err := r.Client.Versions(ctx, project.Slug, r.Server.Loaders(), r.Server.Conf.Version)
	if err != nil {
		return nil, err
	}

	installed, _ := r.Server.Lock.Find(kind, project.Slug)

	choices := make([]VersionChoice, 0, len(versions))
	for _, version := range versions {
		choice := VersionChoice{
			VersionNumber: version.VersionNumber,
			VersionID:     version.ID,
			Channel:       version.VersionType,
			Installed:     version.ID == installed.VersionID,
		}

		if file, ok := version.Primary(); ok {
			choice.Size = file.Size
		}

		choices = append(choices, choice)
	}

	return choices, nil
}

// Search finds projects that fit this server, for adding one by name rather
// than by slug.
func (r *Resolver) Search(ctx context.Context, query string, limit int) ([]modrinth.Project, error) {
	return r.Client.Search(ctx, query, r.Server.Loaders(), r.Server.Conf.Version, limit)
}
