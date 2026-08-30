package pack

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
	"github.com/limelamp/osmium/internal/scope"
)

// fake is a Modrinth stand-in: a handful of projects with builds and
// dependencies, and the files themselves, so a whole install can be run
// without touching the network.
type fake struct {
	server   *httptest.Server
	requests map[string]int
}

// project is one project in the stand-in, with one build per entry.
type build struct {
	number  string
	channel string
	body    string
	deps    []modrinth.Dependency
}

func newFake(t *testing.T, projects map[string][]build) *fake {
	t.Helper()

	f := &fake{requests: map[string]int{}}
	mux := http.NewServeMux()

	files := map[string]string{} // by sha1

	for slug, builds := range projects {
		versions := make([]modrinth.Version, 0, len(builds))

		for i, b := range builds {
			sum := sha1.Sum([]byte(b.body)) //nolint:gosec // a test fixture
			hash := hex.EncodeToString(sum[:])
			files[hash] = b.body

			file := modrinth.File{
				URL: "/files/" + hash, // made absolute when it is served

				Filename: fmt.Sprintf("%s-%s.jar", slug, b.number),
				Primary:  true,
				Size:     int64(len(b.body)),
			}
			file.Hashes.SHA1 = hash

			channel := b.channel
			if channel == "" {
				channel = modrinth.Release
			}

			versions = append(versions, modrinth.Version{
				ID:            slug + "-" + b.number,
				ProjectID:     slug,
				VersionNumber: b.number,
				VersionType:   channel,
				GameVersions:  []string{"1.21.1"},
				Loaders:       []string{"fabric"},
				DatePublished: time.Now().Add(-time.Duration(i) * time.Hour),
				Files:         []modrinth.File{file},
				Dependencies:  b.deps,
			})
		}

		mux.HandleFunc("/project/"+slug, func(w http.ResponseWriter, r *http.Request) {
			f.requests[r.URL.Path]++
			writeJSON(w, modrinth.Project{ID: slug, Slug: slug, Title: strings.ToTitle(slug), ProjectType: "mod"})
		})

		mux.HandleFunc("/project/"+slug+"/version", func(w http.ResponseWriter, r *http.Request) {
			f.requests["versions:"+slug]++
			writeJSON(w, absolute(versions, "http://"+r.Host))
		})
	}

	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/files/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	})

	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		f.requests["projects"]++

		var ids []string
		_ = json.Unmarshal([]byte(r.URL.Query().Get("ids")), &ids)

		found := make([]modrinth.Project, 0, len(ids))
		for _, id := range ids {
			if _, known := projects[id]; known {
				found = append(found, modrinth.Project{ID: id, Slug: id, Title: strings.ToTitle(id)})
			}
		}
		writeJSON(w, found)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.requests["missing:"+r.URL.Path]++
		w.WriteHeader(http.StatusNotFound)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	return f
}

// absolute fills in the base of every download URL, which is only known once
// the stand-in is listening.
func absolute(versions []modrinth.Version, base string) []modrinth.Version {
	out := make([]modrinth.Version, 0, len(versions))
	for _, version := range versions {
		files := make([]modrinth.File, 0, len(version.Files))
		for _, file := range version.Files {
			file.URL = base + file.URL
			files = append(files, file)
		}

		version.Files = files
		out = append(out, version)
	}
	return out
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// world builds a server directory and a resolver pointed at the stand-in.
func world(t *testing.T, projects map[string][]build, wants config.Wants) (resolver *Resolver, dir string) {
	t.Helper()

	f := newFake(t, projects)

	dir = t.TempDir()
	conf := &config.OsmiumConfig{
		Category: "modded", Loader: "fabric", Version: "1.21.1", AllocatedRAM: "4G",
		Mods: wants, Plugins: config.Wants{},
	}
	if err := config.WriteConfigIn(dir, conf); err != nil {
		t.Fatalf("could not write the manifest: %v", err)
	}

	server, err := Open(dir)
	if err != nil {
		t.Fatalf("could not open the server: %v", err)
	}

	return &Resolver{
		Client: modrinth.NewAt(f.server.URL, nil),
		Server: server,
		Cache:  cache.At(filepath.Join(dir, ".cache")),
	}, dir
}

// The graph is walked once per project however many things point at it, and
// everything required comes along with what was asked for.
func TestAddPullsInWhatIsRequired(t *testing.T) {
	resolver, dir := world(t, map[string][]build{
		"spectrum": {{number: "1.10.5", body: "spectrum", deps: []modrinth.Dependency{
			{ProjectID: "fabric-api", Type: modrinth.Required},
			{ProjectID: "owo-lib", Type: modrinth.Required},
			{ProjectID: "sodium", Type: modrinth.Optional},
		}}},
		"fabric-api": {{number: "0.102.0", body: "fabric-api"}},
		"owo-lib": {{number: "0.12.6", body: "owo-lib", deps: []modrinth.Dependency{
			{ProjectID: "fabric-api", Type: modrinth.Required}, // the diamond
		}}},
		"sodium": {{number: "0.5.11", body: "sodium"}},
	}, config.Wants{})

	plan, err := resolver.Add(context.Background(), config.Mods, []string{"spectrum"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}

	if plan.Blocked() {
		t.Fatalf("the plan is blocked: %v", plan.Conflicts)
	}

	if counts := plan.Counts(); counts[Install] != 3 {
		t.Errorf("the plan installs %d projects, want spectrum and its two libraries:\n%s",
			counts[Install], strings.Join(plan.Lines(), "\n"))
	}

	if len(plan.Optional) != 1 || plan.Optional[0].Slug != "sodium" {
		t.Errorf("the optional dependency was not offered: %+v", plan.Optional)
	}

	// The dependency is recorded as one, and the mod asked for as explicit.
	for _, change := range plan.Changes {
		explicit := change.Entry.Explicit
		if change.Slug == "spectrum" != explicit {
			t.Errorf("%s is marked explicit=%v", change.Slug, explicit)
		}
	}

	if err := resolver.Server.Apply(context.Background(), plan, Options{
		Cache: resolver.Cache, Scope: scope.Scope{Dir: dir},
	}); err != nil {
		t.Fatalf("applying the plan failed: %v", err)
	}

	for _, name := range []string{"spectrum-1.10.5.jar", "fabric-api-0.102.0.jar", "owo-lib-0.12.6.jar"} {
		if _, err := os.Stat(filepath.Join(dir, "mods", name)); err != nil {
			t.Errorf("%s was not installed", name)
		}
	}

	// The manifest names only what was asked for; the lock names everything.
	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		t.Fatalf("the manifest is unreadable: %v", err)
	}
	if len(conf.Mods) != 1 || conf.Mods["spectrum"] != config.Newest {
		t.Errorf("the manifest reads %v, want spectrum alone", conf.Mods)
	}

	lock, err := config.ReadLockIn(dir)
	if err != nil {
		t.Fatalf("the lock is unreadable: %v", err)
	}
	if len(lock.Packages) != 3 {
		t.Errorf("the lock holds %d packages, want 3", len(lock.Packages))
	}
}

// Removing a mod takes the libraries that were only there for it, and leaves
// the ones something else still needs.
func TestRemoveTakesOrphansAndKeepsTheRest(t *testing.T) {
	projects := map[string][]build{
		"spectrum": {{number: "1.10.5", body: "spectrum", deps: []modrinth.Dependency{
			{ProjectID: "fabric-api", Type: modrinth.Required},
			{ProjectID: "owo-lib", Type: modrinth.Required},
		}}},
		"sodium": {{number: "0.5.11", body: "sodium", deps: []modrinth.Dependency{
			{ProjectID: "fabric-api", Type: modrinth.Required},
		}}},
		"fabric-api": {{number: "0.102.0", body: "fabric-api"}},
		"owo-lib":    {{number: "0.12.6", body: "owo-lib"}},
	}

	resolver, dir := world(t, projects, config.Wants{})
	ctx := context.Background()

	plan, err := resolver.Add(ctx, config.Mods, []string{"spectrum", "sodium"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}
	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying failed: %v", err)
	}

	plan, err = resolver.Remove(ctx, config.Mods, []string{"spectrum"})
	if err != nil {
		t.Fatalf("the removal failed to plan: %v", err)
	}

	removed := map[string]bool{}
	for _, change := range plan.Changes {
		if change.Action == Remove {
			removed[change.Slug] = true
		}
	}

	if !removed["spectrum"] || !removed["owo-lib"] {
		t.Errorf("the removal leaves %v behind; owo-lib is needed by nothing now:\n%s",
			removed, strings.Join(plan.Lines(), "\n"))
	}
	if removed["fabric-api"] {
		t.Error("fabric-api was removed although sodium still needs it")
	}

	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying the removal failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mods", "owo-lib-0.12.6.jar")); err == nil {
		t.Error("the orphaned library is still on disk")
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "fabric-api-0.102.0.jar")); err != nil {
		t.Error("a library something still needs was deleted")
	}
}

// Two mods that declare each other used to be followed until the stack ran
// out. The graph is walked once per project, so a cycle simply ends.
func TestACycleResolves(t *testing.T) {
	resolver, _ := world(t, map[string][]build{
		"alpha": {{number: "1.0", body: "alpha", deps: []modrinth.Dependency{
			{ProjectID: "beta", Type: modrinth.Required},
		}}},
		"beta": {{number: "1.0", body: "beta", deps: []modrinth.Dependency{
			{ProjectID: "alpha", Type: modrinth.Required},
		}}},
	}, config.Wants{})

	done := make(chan Plan, 1)
	go func() {
		plan, err := resolver.Add(context.Background(), config.Mods, []string{"alpha"})
		if err != nil {
			t.Errorf("the plan failed: %v", err)
		}
		done <- plan
	}()

	select {
	case plan := <-done:
		if counts := plan.Counts(); counts[Install] != 2 {
			t.Errorf("a cycle of two mods planned %d installs, want 2", counts[Install])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("resolving a dependency cycle did not finish")
	}
}

// A release is preferred, and a pre-release is taken only when a project has
// no release for this server at all — with the plan saying so.
func TestPreReleasesAreLastResort(t *testing.T) {
	resolver, _ := world(t, map[string][]build{
		"steady": {
			{number: "2.0-beta", channel: modrinth.Beta, body: "steady-beta"},
			{number: "1.9", body: "steady-release"},
		},
		"betaonly": {{number: "2.3.0-beta", channel: modrinth.Beta, body: "betaonly"}},
	}, config.Wants{})

	plan, err := resolver.Add(context.Background(), config.Mods, []string{"steady", "betaonly"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}

	chosen := map[string]string{}
	for _, change := range plan.Changes {
		chosen[change.Slug] = change.To
	}

	if chosen["steady"] != "1.9" {
		t.Errorf("a beta was chosen over a release: %s", chosen["steady"])
	}
	if chosen["betaonly"] != "2.3.0-beta" {
		t.Errorf("a mod with only pre-releases was not installed: %s", chosen["betaonly"])
	}
	if len(plan.Notes) == 0 {
		t.Error("taking a pre-release was not mentioned in the plan")
	}
}

// A pin names one exact build, and holds it while everything else moves.
func TestAPinHoldsItsBuild(t *testing.T) {
	projects := map[string][]build{
		"sodium": {
			{number: "0.6.0", body: "sodium-new"},
			{number: "0.5.8", body: "sodium-old"},
		},
	}

	resolver, dir := world(t, projects, config.Wants{})
	ctx := context.Background()

	plan, err := resolver.Add(ctx, config.Mods, []string{"sodium@0.5.8"})
	if err != nil {
		t.Fatalf("the pin failed to plan: %v", err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].To != "0.5.8" {
		t.Fatalf("the pinned build was not chosen: %s", strings.Join(plan.Lines(), "\n"))
	}

	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying failed: %v", err)
	}

	// An update leaves it alone and says why.
	plan, err = resolver.Update(ctx, config.Mods, nil)
	if err != nil {
		t.Fatalf("the update failed to plan: %v", err)
	}
	if !plan.Empty() {
		t.Errorf("the update moved a pinned mod:\n%s", strings.Join(plan.Lines(), "\n"))
	}

	// Unpinning lets it move again.
	plan, err = resolver.Unpin(ctx, config.Mods, "sodium")
	if err != nil {
		t.Fatalf("unpinning failed: %v", err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Action != Upgrade || plan.Changes[0].To != "0.6.0" {
		t.Errorf("unpinning did not free the mod:\n%s", strings.Join(plan.Lines(), "\n"))
	}

	if _, err := os.Stat(filepath.Join(dir, "mods", "sodium-0.5.8.jar")); err != nil {
		t.Error("the pinned build is not the one on disk")
	}
}

// A jar Osmium did not install is never touched, and is reported so it can be
// adopted deliberately.
func TestUntrackedFilesAreLeftAlone(t *testing.T) {
	resolver, dir := world(t, map[string][]build{
		"sodium": {{number: "0.6.0", body: "sodium"}},
	}, config.Wants{})

	mods := filepath.Join(dir, "mods")
	if err := os.MkdirAll(mods, 0o750); err != nil {
		t.Fatal(err)
	}

	byHand := filepath.Join(mods, "hand-built.jar")
	if err := os.WriteFile(byHand, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	plan, err := resolver.Add(ctx, config.Mods, []string{"sodium"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}

	if len(plan.Untracked) != 1 || plan.Untracked[0] != "hand-built.jar" {
		t.Errorf("the untracked jar was not reported: %v", plan.Untracked)
	}

	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying failed: %v", err)
	}

	if _, err := os.Stat(byHand); err != nil {
		t.Error("a jar Osmium did not install was removed")
	}
}

// A download that goes wrong leaves the server exactly as it was.
func TestAFailedPlanChangesNothing(t *testing.T) {
	resolver, dir := world(t, map[string][]build{
		"good": {{number: "1.0", body: "good"}},
		"bad":  {{number: "1.0", body: "bad"}},
	}, config.Wants{})

	ctx := context.Background()

	// Install one mod, so there is a state to be left alone.
	plan, err := resolver.Add(ctx, config.Mods, []string{"good"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}
	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying failed: %v", err)
	}

	before, err := os.ReadFile(filepath.Join(dir, config.LockFileName))
	if err != nil {
		t.Fatal(err)
	}

	// Now a plan whose download cannot succeed.
	plan, err = resolver.Add(ctx, config.Mods, []string{"bad"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}
	for i := range plan.Changes {
		plan.Changes[i].Entry.DownloadURL = "http://127.0.0.1:9/nothing-here"
	}

	if err := resolver.Server.Apply(ctx, plan, Options{Cache: cache.Off()}); err == nil {
		t.Fatal("a download that cannot work reported success")
	}

	after, err := os.ReadFile(filepath.Join(dir, config.LockFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a failed plan changed the lock")
	}

	if _, err := os.Stat(filepath.Join(dir, "mods", "good-1.0.jar")); err != nil {
		t.Error("a failed plan disturbed what was already installed")
	}
	if _, err := os.Stat(filepath.Join(dir, "mods", "bad-1.0.jar")); err == nil {
		t.Error("a failed plan left a file behind")
	}
}

// The second install of the same file costs no network at all.
func TestTheCacheSavesTheDownload(t *testing.T) {
	resolver, dir := world(t, map[string][]build{
		"sodium": {{number: "0.6.0", body: "sodium"}},
	}, config.Wants{})

	ctx := context.Background()
	plan, err := resolver.Add(ctx, config.Mods, []string{"sodium"})
	if err != nil {
		t.Fatalf("the plan failed: %v", err)
	}
	if err := resolver.Server.Apply(ctx, plan, Options{Cache: resolver.Cache}); err != nil {
		t.Fatalf("applying failed: %v", err)
	}

	// Delete the file from the server and plan the repair: the same build is
	// wanted, and the cache already has it.
	if err := os.Remove(filepath.Join(dir, "mods", "sodium-0.6.0.jar")); err != nil {
		t.Fatal(err)
	}

	server, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	resolver.Server = server

	plan, err = resolver.Sync(ctx, config.Mods)
	if err != nil {
		t.Fatalf("the repair failed to plan: %v", err)
	}

	if len(plan.Changes) != 1 || plan.Changes[0].Action != Reinstall {
		t.Fatalf("a missing file was not noticed:\n%s", strings.Join(plan.Lines(), "\n"))
	}
	if !plan.Changes[0].Cached {
		t.Error("the file is in the cache but the plan wants to download it again")
	}
	if plan.Download() != 0 {
		t.Errorf("the plan would download %d bytes it already has", plan.Download())
	}
}

// A manifest written before there were lock files still describes a server,
// and what it recorded becomes the lock.
func TestAnOldManifestIsUnderstood(t *testing.T) {
	dir := t.TempDir()

	old := `{
	 "category": "modded", "loader": "fabric", "version": "1.21.1", "ram": "4G",
	 "mods": {
	  "sodium": {"version_number": "0.5.11", "filename": "sodium-0.5.11.jar",
	             "url": "https://example.invalid/sodium.jar", "sha1": "abc123"}
	 },
	 "plugins": {}
	}`

	if err := os.WriteFile(filepath.Join(dir, config.OsmiumFileName), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	server, err := Open(dir)
	if err != nil {
		t.Fatalf("an old manifest could not be read: %v", err)
	}

	if want := server.Conf.Mods["sodium"]; want != config.Newest {
		t.Errorf("the old entry reads %q, want %q", want, config.Newest)
	}

	entry, locked := server.Lock.Find(config.Mods, "sodium")
	if !locked {
		t.Fatal("what the old manifest said was installed was not carried into the lock")
	}
	if entry.VersionNumber != "0.5.11" || entry.FileName != "sodium-0.5.11.jar" || !entry.Explicit {
		t.Errorf("the seeded lock entry is wrong: %+v", entry)
	}
}
