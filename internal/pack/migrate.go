package pack

import (
	"context"
	"fmt"
	"strings"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/util"
)

// Migration is what moving a server to another engine or Minecraft version
// would do, worked out before any of it happens.
type Migration struct {
	From, To string

	// Plans is the work per kind: what survives the move, at which build.
	Plans map[config.Kind]Plan

	// Lifted are the pins that could not come across, and Stranded are the
	// projects with nothing published for where the server is going. Both are
	// reported rather than silently dropped: a migration that quietly removes
	// half a modpack is worse than one that says what it cannot bring.
	Lifted   []string
	Stranded []string
}

// Summary is the migration in a line.
func (m Migration) Summary() string {
	var moved, removed int
	for _, plan := range m.Plans {
		counts := plan.Counts()
		moved += counts[Install] + counts[Upgrade] + counts[Downgrade] + counts[Reinstall]
		removed += counts[Remove]
	}

	return fmt.Sprintf("%s → %s   %d to move, %d left behind", m.From, m.To, moved, removed)
}

// PlanMigration works out what moving a server to another loader or Minecraft
// version would take.
//
// Pins are lifted rather than obeyed when the build they name does not exist
// for the new version: a pin says "this build, until I say otherwise", and a
// server that cannot start is not what anyone meant by it. What was lifted is
// reported so it can be pinned again deliberately.
func PlanMigration(ctx context.Context, resolve *Resolver, loader, version string) (Migration, error) {
	server := resolve.Server

	migration := Migration{
		From:  strings.TrimSpace(server.Conf.Loader + " " + server.Conf.Version),
		To:    strings.TrimSpace(loader + " " + version),
		Plans: map[config.Kind]Plan{},
	}

	// The resolver works against the server as it will be, not as it is.
	server.Conf.Loader, server.Conf.Version = loader, version

	for _, kind := range []config.Kind{config.Mods, config.Plugins} {
		wants := server.Conf.Want(kind)
		if len(wants) == 0 {
			continue
		}

		lifted, stranded := resolve.reconcile(ctx, wants)
		migration.Lifted = append(migration.Lifted, lifted...)
		migration.Stranded = append(migration.Stranded, stranded...)

		plan, err := resolve.Sync(ctx, kind)
		if err != nil {
			return migration, err
		}

		migration.Plans[kind] = plan
	}

	return migration, nil
}

// reconcile makes a set of wants answerable for the server as it will be:
// pins that have no build there are lifted, and projects with nothing at all
// are taken out of the manifest.
func (r *Resolver) reconcile(ctx context.Context, wants config.Wants) (lifted, stranded []string) {
	for _, slug := range wants.Slugs() {
		want := wants[slug]

		versions, err := r.Client.Versions(ctx, slug, r.Server.Loaders(), r.Server.Conf.Version)
		if err != nil || len(versions) == 0 {
			delete(wants, slug)
			stranded = append(stranded, fmt.Sprintf("%s has no build for %s %s",
				slug, r.Server.Conf.Loader, r.Server.Conf.Version))
			continue
		}

		if !want.Pinned() {
			continue
		}

		held := false
		for _, version := range versions {
			if matchesBuild(version, string(want)) {
				held = true
				break
			}
		}

		if !held {
			wants[slug] = config.Newest
			lifted = append(lifted, fmt.Sprintf("%s was pinned at %s, which has no %s build — taking %s instead",
				slug, want, r.Server.Conf.Version, versions[0].VersionNumber))
		}
	}

	return lifted, stranded
}

// Apply carries out a migration: the new engine's jar first, then the mods and
// plugins that came across with it.
//
// The jar goes first deliberately. The manifest is only written once the files
// it describes are in place, so a failed download leaves a server that still
// says what it actually is.
func (m Migration) Apply(ctx context.Context, server *Server, opts Options, sc scope.Scope) error {
	sc.Logf("migrating %s", m.Summary())

	for _, note := range m.Lifted {
		sc.Logf("  %s", note)
	}
	for _, note := range m.Stranded {
		sc.Logf("  %s", note)
	}

	if err := util.DownloadJarInto(sc, server.Conf.Loader, server.Conf.Version); err != nil {
		return fmt.Errorf("failed to download the new server files: %w", err)
	}

	for _, kind := range []config.Kind{config.Mods, config.Plugins} {
		plan, planned := m.Plans[kind]
		if !planned {
			continue
		}

		if err := server.Apply(ctx, plan, opts); err != nil {
			return err
		}
	}

	// A server with nothing installed still has a new engine to record.
	return server.Save()
}

// MigrateInPlace opens a server, works out the move and carries it out. It is
// the whole of what the migrate command and the migrate wizard do, so the two
// cannot drift apart.
func MigrateInPlace(ctx context.Context, dir, loader, version string, sc scope.Scope, store *cache.Store) error {
	server, err := Open(dir)
	if err != nil {
		return err
	}

	resolve := &Resolver{Client: modrinth.New(store), Server: server, Cache: store}

	migration, err := PlanMigration(ctx, resolve, loader, version)
	if err != nil {
		return err
	}

	return migration.Apply(ctx, server, Options{Cache: store, Scope: sc}, sc)
}
