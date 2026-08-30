package pack

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
)

// depthLimit stops a resolution that has gone wrong from running away. A real
// dependency graph is a handful deep; a hundred means something is looping in
// a way the cycle guard did not catch.
const depthLimit = 100

// Resolver turns what a server asks for into exactly which files it should
// have, and what it would take to get there.
type Resolver struct {
	Client *modrinth.Client
	Server *Server
	Cache  *cache.Store

	// Prerelease allows a beta or alpha build to be chosen even when a release
	// exists. Without it, a pre-release is only ever taken when a project has
	// no release at all for this server, which is common enough on a new
	// Minecraft version that refusing outright would be useless.
	Prerelease bool
}

// request is one project the resolution still has to look at.
type request struct {
	ref      string // a slug, a project id, or a version id
	want     config.Want
	explicit bool
	parent   string
	depth    int
}

// resolution is the finished graph.
type resolution struct {
	entries   map[string]config.Locked
	order     map[string]int // where each choice sat in the list of builds
	optional  []Suggestion
	notes     []string
	conflicts []string
}

// Add brings projects into the manifest and plans what that would take.
// A reference is a slug, optionally with the build to hold it at: sodium,
// sodium@0.5.8.
func (r *Resolver) Add(ctx context.Context, kind config.Kind, refs []string) (Plan, error) {
	wants := r.copyWants(kind)

	for _, ref := range refs {
		slug, build := SplitRef(ref)
		if slug == "" {
			continue
		}

		// The slug given on the command line may be an id, or the wrong case;
		// what goes in the manifest is what Modrinth calls the project.
		project, err := r.Client.Project(ctx, slug)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", slug, err)
		}

		wants[project.Slug] = config.Want(build)
		if build == "" {
			wants[project.Slug] = config.Newest
		}
	}

	return r.planFor(ctx, kind, wants)
}

// Remove takes projects out of the manifest, along with anything that was only
// there for them.
func (r *Resolver) Remove(ctx context.Context, kind config.Kind, slugs []string) (Plan, error) {
	wants := r.copyWants(kind)

	for _, slug := range slugs {
		key := r.resolveSlug(kind, slug)
		if _, asked := wants[key]; !asked {
			if _, locked := r.Server.Lock.Find(kind, key); !locked {
				return Plan{}, fmt.Errorf("%s is not installed", slug)
			}
		}
		delete(wants, key)
	}

	return r.planFor(ctx, kind, wants)
}

// Update moves projects to the newest build that fits, leaving pinned ones
// alone. With no names it considers everything.
func (r *Resolver) Update(ctx context.Context, kind config.Kind, slugs []string) (Plan, error) {
	wants := r.copyWants(kind)

	chosen := slugs
	if len(chosen) == 0 {
		chosen = wants.Slugs()
	}

	var notes []string
	for _, slug := range chosen {
		key := r.resolveSlug(kind, slug)

		want, asked := wants[key]
		if !asked {
			return Plan{}, fmt.Errorf("%s is not installed", slug)
		}

		if want.Pinned() {
			notes = append(notes, fmt.Sprintf("%s is pinned at %s, left alone", key, want))
		}
	}

	plan, err := r.planFor(ctx, kind, wants)
	if err != nil {
		return Plan{}, err
	}

	plan.Notes = append(notes, plan.Notes...)
	return plan, nil
}

// Sync plans what it would take to make the server hold exactly what it asks
// for — the command that rebuilds a server from a manifest someone handed you,
// and the one that repairs a folder somebody has been deleting files from.
func (r *Resolver) Sync(ctx context.Context, kind config.Kind) (Plan, error) {
	return r.planFor(ctx, kind, r.copyWants(kind))
}

// Pin holds a project at the build it currently has, or at a named one.
func (r *Resolver) Pin(ctx context.Context, kind config.Kind, slug, build string) (Plan, error) {
	key := r.resolveSlug(kind, slug)

	entry, locked := r.Server.Lock.Find(kind, key)
	if !locked && build == "" {
		return Plan{}, fmt.Errorf("%s is not installed, so there is no build to pin it at", slug)
	}

	if build == "" {
		build = entry.VersionNumber
	}

	wants := r.copyWants(kind)
	wants[key] = config.Want(build)

	return r.planFor(ctx, kind, wants)
}

// Unpin lets a project move with the rest again.
func (r *Resolver) Unpin(ctx context.Context, kind config.Kind, slug string) (Plan, error) {
	key := r.resolveSlug(kind, slug)

	wants := r.copyWants(kind)
	if _, asked := wants[key]; !asked {
		return Plan{}, fmt.Errorf("%s is not installed", slug)
	}

	wants[key] = config.Newest
	return r.planFor(ctx, kind, wants)
}

// Wanted is the manifest as the plan would leave it, which is what Apply
// writes once the plan has been carried out.
type Wanted struct {
	Kind  config.Kind
	Wants config.Wants
}

// planFor resolves a set of wants and works out what it would take to get
// there from where the server is now.
func (r *Resolver) planFor(ctx context.Context, kind config.Kind, wants config.Wants) (Plan, error) {
	resolved, err := r.resolve(ctx, kind, wants)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		Optional:  resolved.optional,
		Notes:     resolved.notes,
		Conflicts: resolved.conflicts,
		Untracked: r.Server.Untracked(kind),
	}

	plan.Changes = r.diff(kind, resolved)
	sortChanges(plan.Changes)

	// The manifest the plan would leave behind travels with it, so applying it
	// records exactly what was agreed to rather than re-deriving it.
	plan.wants = &Wanted{Kind: kind, Wants: wants}

	return plan, nil
}

// resolve walks the dependency graph breadth-first from what was asked for.
//
// Breadth-first rather than depth-first so that the shortest reason for a
// project being present is the one recorded, and so a diamond — two mods
// needing the same library — is looked up once rather than once per path.
func (r *Resolver) resolve(ctx context.Context, kind config.Kind, wants config.Wants) (resolution, error) {
	out := resolution{
		entries: make(map[string]config.Locked, len(wants)),
		order:   make(map[string]int, len(wants)),
	}

	queue := make([]request, 0, len(wants))
	for _, slug := range wants.Slugs() {
		queue = append(queue, request{ref: slug, want: wants[slug], explicit: true})
	}

	// seen guards against a cycle: two mods that each declare the other, which
	// a naive walk follows until it runs out of stack.
	seen := make(map[string]bool, len(wants))
	suggested := make(map[string]Suggestion)

	for len(queue) > 0 {
		req := queue[0]
		queue = queue[1:]

		if req.depth > depthLimit {
			out.conflicts = append(out.conflicts,
				fmt.Sprintf("the dependencies of %s go round in circles", req.ref))
			continue
		}

		project, err := r.Client.Project(ctx, req.ref)
		if err != nil {
			if errors.Is(err, modrinth.ErrNotFound) {
				out.conflicts = append(out.conflicts, fmt.Sprintf("%s is not on Modrinth", req.ref))
				continue
			}
			return out, err
		}

		if seen[project.ID] {
			// Already resolved by another path; record the extra reason.
			if entry, ok := out.entries[project.Slug]; ok && req.parent != "" {
				entry.RequiredBy = addOnce(entry.RequiredBy, req.parent)
				entry.Explicit = entry.Explicit || req.explicit
				out.entries[project.Slug] = entry
			}
			continue
		}
		seen[project.ID] = true

		version, index, note, err := r.choose(ctx, project, req)
		if err != nil {
			out.conflicts = append(out.conflicts, err.Error())
			continue
		}
		if note != "" {
			out.notes = append(out.notes, note)
		}

		file, ok := version.Primary()
		if !ok {
			out.conflicts = append(out.conflicts,
				fmt.Sprintf("%s %s has no file to download", project.Slug, version.VersionNumber))
			continue
		}

		entry := config.Locked{
			Slug:          project.Slug,
			Kind:          kind,
			ProjectID:     project.ID,
			VersionID:     version.ID,
			VersionNumber: version.VersionNumber,
			VersionType:   version.VersionType,
			FileName:      file.Filename,
			DownloadURL:   file.URL,
			SHA1:          file.Hashes.SHA1,
			Size:          file.Size,
			Explicit:      req.explicit,
			Pinned:        req.want.Pinned(),
		}
		if req.parent != "" {
			entry.RequiredBy = []string{req.parent}
		}

		out.entries[project.Slug] = entry
		out.order[project.Slug] = index

		for _, dep := range version.Dependencies {
			switch dep.Type {
			case modrinth.Required:
				ref := dep.ProjectID
				if ref == "" {
					ref = dep.VersionID
				}
				if ref == "" {
					continue
				}

				queue = append(queue, request{
					ref: ref, want: config.Want(dep.VersionID), explicit: false,
					parent: project.Slug, depth: req.depth + 1,
				})

			case modrinth.Optional:
				if dep.ProjectID != "" {
					suggested[dep.ProjectID] = Suggestion{Slug: dep.ProjectID, Wanted: project.Slug}
				}

			case modrinth.Incompatible:
				if dep.ProjectID != "" {
					out.conflicts = append(out.conflicts, fmt.Sprintf(
						"%s says it cannot run beside %s", project.Slug, dep.ProjectID))
				}

			case modrinth.Embedded:
				// Already inside the jar; fetching it separately would put two
				// copies of the same library on the classpath.
			}
		}
	}

	out.optional = r.nameSuggestions(ctx, suggested, out.entries)
	return out, nil
}

// choose picks the build to install: the pinned one if there is a pin, and
// otherwise the newest that fits.
//
// Minecraft mod version numbers are free text, so "newest" can only mean most
// recently published — which is what Modrinth orders by and what a person
// means when they say it.
func (r *Resolver) choose(ctx context.Context, project modrinth.Project, req request) (chosen modrinth.Version, order int, note string, err error) {
	// A dependency may name the exact build it needs, which beats any search.
	if !req.explicit && req.want.Pinned() {
		if version, err := r.Client.Version(ctx, string(req.want)); err == nil {
			return version, 0, "", nil
		}
	}

	versions, err := r.Client.Versions(ctx, project.Slug, r.Server.Loaders(), r.Server.Conf.Version)
	if err != nil {
		return modrinth.Version{}, 0, "", err
	}

	if len(versions) == 0 {
		return modrinth.Version{}, 0, "", fmt.Errorf("%s has no build for %s %s",
			project.Slug, r.Server.Conf.Loader, r.Server.Conf.Version)
	}

	if req.want.Pinned() {
		for index, version := range versions {
			if matchesBuild(version, string(req.want)) {
				return version, index, "", nil
			}
		}

		return modrinth.Version{}, 0, "", fmt.Errorf("%s has no build %q for %s %s (it has %s)",
			project.Slug, req.want, r.Server.Conf.Loader, r.Server.Conf.Version, listBuilds(versions))
	}

	if !r.Prerelease {
		for index, version := range versions {
			if version.Stable() {
				return version, index, "", nil
			}
		}

		// Nothing but pre-releases: take the newest and say so, rather than
		// refusing to install a mod that simply has not had a release since
		// this Minecraft version came out.
		return versions[0], 0, fmt.Sprintf("%s has no release build for %s, taking %s (%s)",
			project.Slug, r.Server.Conf.Version, versions[0].VersionNumber, versions[0].VersionType), nil
	}

	return versions[0], 0, "", nil
}

// nameSuggestions turns optional dependencies into something worth printing,
// dropping any that are already installed. The titles are fetched in one
// request rather than one each.
func (r *Resolver) nameSuggestions(ctx context.Context, suggested map[string]Suggestion, resolved map[string]config.Locked) []Suggestion {
	if len(suggested) == 0 {
		return nil
	}

	ids := make([]string, 0, len(suggested))
	for id := range suggested {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	projects, err := r.Client.Projects(ctx, ids)
	if err != nil {
		projects = nil
	}

	out := make([]Suggestion, 0, len(ids))
	for _, id := range ids {
		suggestion := suggested[id]

		if project, known := projects[strings.ToLower(id)]; known {
			suggestion.Slug, suggestion.Title = project.Slug, project.Title
		}

		// Something already in the graph is not a suggestion.
		if _, installed := resolved[suggestion.Slug]; installed {
			continue
		}

		out = append(out, suggestion)
	}

	return out
}

// diff works out what has to happen to get from what the server has to what it
// should have.
func (r *Resolver) diff(kind config.Kind, resolved resolution) []Change {
	var changes []Change

	slugs := make([]string, 0, len(resolved.entries))
	for slug := range resolved.entries {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		target := resolved.entries[slug]
		change := Change{
			Kind: kind, Slug: slug, To: target.VersionNumber,
			Entry: target, Size: target.Size, Why: reasonFor(target),
			Cached: r.Cache.Has(target.SHA1),
		}

		existing, installed := r.Server.Lock.Find(kind, slug)
		switch {
		case !installed:
			change.Action = Install

		case existing.VersionID != target.VersionID:
			change.Action = Upgrade
			change.From = existing.VersionNumber

			// The list of builds is newest first, so a target further down it
			// than what is installed is a step backwards.
			if index, known := resolved.order[slug]; known && index > 0 && existing.VersionNumber != "" {
				if r.olderThanInstalled(kind, slug, index) {
					change.Action = Downgrade
				}
			}

		default:
			// The lock and the target agree; the only thing that can still be
			// wrong is the file itself being gone.
			if _, present := r.Server.Installed(existing); present {
				continue
			}
			change.Action = Reinstall
			change.From = existing.VersionNumber
		}

		changes = append(changes, change)
	}

	for _, entry := range r.Server.Lock.Of(kind) {
		if _, kept := resolved.entries[entry.Slug]; kept {
			continue
		}

		why := "no longer needed"
		if entry.Explicit {
			why = "asked to remove"
		}

		changes = append(changes, Change{
			Action: Remove, Kind: kind, Slug: entry.Slug,
			From: entry.VersionNumber, Entry: entry, Why: why,
		})
	}

	return changes
}

// olderThanInstalled reports whether the chosen build sits below the installed
// one in the list of builds, which is what makes a change a downgrade.
func (r *Resolver) olderThanInstalled(kind config.Kind, slug string, chosen int) bool {
	existing, ok := r.Server.Lock.Find(kind, slug)
	if !ok {
		return false
	}

	versions, err := r.Client.Versions(context.Background(), slug, r.Server.Loaders(), r.Server.Conf.Version)
	if err != nil {
		return false
	}

	for index, version := range versions {
		if version.ID == existing.VersionID {
			return chosen > index
		}
	}
	return false
}

// copyWants takes the manifest's set for a kind, so a plan can be worked out
// without changing anything until it is applied.
func (r *Resolver) copyWants(kind config.Kind) config.Wants {
	source := r.Server.Conf.Want(kind)

	wants := make(config.Wants, len(source))
	for slug, want := range source {
		wants[slug] = want
	}
	return wants
}

// resolveSlug matches what the user typed against what is installed, so a
// project can be named by its id, its slug, or the wrong case.
func (r *Resolver) resolveSlug(kind config.Kind, name string) string {
	name = strings.TrimSpace(name)

	for _, entry := range r.Server.Lock.Of(kind) {
		if strings.EqualFold(entry.Slug, name) || strings.EqualFold(entry.ProjectID, name) {
			return entry.Slug
		}
	}

	for slug := range r.Server.Conf.Want(kind) {
		if strings.EqualFold(slug, name) {
			return slug
		}
	}

	return name
}

// SplitRef divides "sodium@0.5.8" into the project and the build it names.
func SplitRef(ref string) (slug, build string) {
	ref = strings.TrimSpace(ref)

	if at := strings.LastIndex(ref, "@"); at > 0 {
		return ref[:at], ref[at+1:]
	}
	return ref, ""
}

// matchesBuild reports whether a version answers to the name given: its build
// number, its Modrinth id, or an unambiguous beginning of either.
func matchesBuild(version modrinth.Version, build string) bool {
	switch {
	case strings.EqualFold(version.VersionNumber, build), strings.EqualFold(version.ID, build):
		return true
	case len(build) >= 3 && strings.HasPrefix(strings.ToLower(version.VersionNumber), strings.ToLower(build)):
		return true
	}
	return false
}

// listBuilds names the newest few builds, for an error that has to say what
// was available instead.
func listBuilds(versions []modrinth.Version) string {
	names := make([]string, 0, 5)
	for _, version := range versions[:min(len(versions), 5)] {
		names = append(names, version.VersionNumber)
	}

	list := strings.Join(names, ", ")
	if len(versions) > len(names) {
		list += ", …"
	}
	return list
}

// reasonFor says why a project is in the plan.
func reasonFor(entry config.Locked) string {
	switch {
	case entry.Explicit && entry.Pinned:
		return "pinned"
	case entry.Explicit:
		return ""
	case len(entry.RequiredBy) > 0:
		return "required by " + strings.Join(entry.RequiredBy, ", ")
	default:
		return "required"
	}
}

// addOnce appends a name that is not already in the list.
func addOnce(list []string, name string) []string {
	for _, existing := range list {
		if existing == name {
			return list
		}
	}
	return append(list, name)
}

// Restore plans an install from the lock alone, without asking Modrinth
// anything.
//
// This is what makes a manifest reproducible: the lock names the exact builds
// and their hashes, so the same server comes back file for file, and it comes
// back without a network round trip for any file already in the cache. What it
// cannot do is invent a build that has since been deleted from Modrinth, so a
// caller that fails here falls back to resolving.
func (r *Resolver) Restore(kind config.Kind) Plan {
	plan := Plan{Untracked: r.Server.Untracked(kind)}

	for _, entry := range r.Server.Lock.Of(kind) {
		if _, present := r.Server.Installed(entry); present {
			continue
		}

		plan.Changes = append(plan.Changes, Change{
			Action: Install, Kind: kind, Slug: entry.Slug,
			To: entry.VersionNumber, Entry: entry, Size: entry.Size,
			Why: reasonFor(entry), Cached: r.Cache.Has(entry.SHA1),
		})
	}

	sortChanges(plan.Changes)
	return plan
}

// Locked reports whether a server has a lock to restore from.
func (r *Resolver) Locked(kind config.Kind) bool {
	return len(r.Server.Lock.Of(kind)) > 0
}
