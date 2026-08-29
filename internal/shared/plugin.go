package shared

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/util"
)

//* API didn't work as expected
// type modrinthProject struct {
// 	ProjectType string `json:"project_type"`
// }

/*
	What the dependency types mean
		required — the mod must be present for this version to work.
		optional — the mod can be present but isn’t necessary.
		incompatible — the mod must not be present because it breaks things.
		embedded — the dependency is included inside the mod itself and doesn’t have to be fetched separately.
*/

type projectInfo struct {
	Slug    string   `json:"slug"`
	Loaders []string `json:"loaders"`
}

type dependency struct {
	ProjectID      string `json:"project_id"`
	DependencyType string `json:"dependency_type"`
}

// The structure of the Modrinth Version API response
type modrinthVersion struct {
	ProjectID     string   `json:"project_id"`
	VersionNumber string   `json:"version_number"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	VersionType   string   `json:"version_type"` // "release", "beta", "alpha"
	Files         []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
		Hashes   struct {
			SHA1 string `json:"sha1"`
		} `json:"hashes"`
	} `json:"files"`
	Dependencies []dependency `json:"dependencies"`
}

var modrinthHTTPClient = &http.Client{Timeout: 30 * time.Second}

//# --- project functions ---

// doModrinthRequest performs an HTTP request with required Modrinth headers
func doModrinthRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Osmium-Manager/1.0")
	return modrinthHTTPClient.Do(req)
}

// getProjectInfo fetches the project info (slug and loaders) from Modrinth API by project ID
func getProjectInfo(projectID string) (projectInfo, error) {
	slugUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s", projectID)

	resp, err := doModrinthRequest(slugUrl)
	if err != nil {
		return projectInfo{}, fmt.Errorf("failed to fetch slug: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return projectInfo{}, fmt.Errorf("failed to fetch project info: %s", resp.Status)
	}

	var info projectInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return projectInfo{}, fmt.Errorf("failed to decode slug response: %w", err)
	}

	return info, nil
}

// isProjectInstalled checks if a project is already installed in the config
func isProjectInstalled(sc scope.Scope, slug, folder string, conf *config.OsmiumConfig) bool {
	switch folder {
	case "mods", "optional_mods":
		project, ok := conf.Mods[slug]
		// check if the file exists in mods folder
		if _, err := os.Stat(sc.Path("mods", project.FileName)); os.IsNotExist(err) {
			return false
		}
		return ok
	case "plugins", "optional_plugins":
		project, ok := conf.Plugins[slug]
		// check if the file exists in plugins folder
		if _, err := os.Stat(sc.Path("plugins", project.FileName)); os.IsNotExist(err) {
			return false
		}
		return ok
	default:
		return false
	}
}

// buildVersionQueryURL constructs the Modrinth version query URL with filters
func buildVersionQueryURL(slug string, conf *config.OsmiumConfig) (string, error) {
	baseUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version", slug)

	projectUrl, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse base url: %w", err)
	}

	q := projectUrl.Query()
	q.Set("game_versions", fmt.Sprintf(`["%s"]`, conf.Version))

	// Get compatible loaders
	loaders, ok := constants.PLUGIN_RESOLVER[strings.ToLower(conf.Loader)]
	if !ok {
		loaders = []string{strings.ToLower(conf.Loader)}
	}
	q.Set("loaders", `["`+strings.Join(loaders, `","`)+`"]`)

	projectUrl.RawQuery = q.Encode()
	return projectUrl.String(), nil
}

// getProjectVersions fetches compatible versions from Modrinth API
func getProjectVersions(slug string, conf *config.OsmiumConfig) ([]modrinthVersion, error) {
	projectUrl, err := buildVersionQueryURL(slug, conf)
	if err != nil {
		return nil, err
	}

	resp, err := doModrinthRequest(projectUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch versions: %s", resp.Status)
	}

	var versions []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no compatible versions found for Minecraft %s", conf.Version)
	}

	return versions, nil
}

// downloadFile downloads a file from URL to the specified folder
func downloadFile(sc scope.Scope, url, folder, filename string) error {
	resp, err := modrinthHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	// Ensure folder exists
	if err := os.MkdirAll(sc.Path(folder), 0755); err != nil {
		return fmt.Errorf("failed to create %s folder: %w", folder, err)
	}

	filePath := sc.Path(folder, filename)
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// A mod is small next to a server jar, but on a slow connection it is still
	// worth showing, and it costs one counter.
	counter := &progressCounter{scope: sc, label: filename, total: resp.ContentLength}
	sc.Report(filename, 0, resp.ContentLength)

	if _, err = io.Copy(io.MultiWriter(out, counter), resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	sc.Report(filename, counter.done, counter.total)
	return nil
}

// progressCounter reports how much of a download has arrived.
type progressCounter struct {
	scope scope.Scope
	label string
	total int64
	done  int64
}

func (c *progressCounter) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	c.scope.Report(c.label, c.done, c.total)
	return len(p), nil
}

// updateConfigWithProject adds the project to the appropriate section in config
func updateConfigWithProject(sc scope.Scope, slug, folder string, version modrinthVersion, conf *config.OsmiumConfig) error {
	if len(version.Files) == 0 {
		return fmt.Errorf("no files found in version")
	}

	fileInfo := version.Files[0]
	project := config.Project{
		VersionNumber: version.VersionNumber,
		FileName:      fileInfo.Filename,
		DownloadURL:   fileInfo.URL,
		SHA1:          fileInfo.Hashes.SHA1,
	}

	switch folder {
	case "mods":
		conf.Mods[slug] = project
	case "plugins":
		conf.Plugins[slug] = project
		// optional folders don't get tracked in config
	}

	return config.WriteConfigIn(sc.Dir, conf)
}

// getDependencyFolder determines the correct folder for a dependency
func getDependencyFolder(parentFolder string, depType string) (string, error) {
	if strings.HasPrefix(parentFolder, "optional_") {
		// If parent is optional, all dependencies go to same optional folder
		return parentFolder, nil
	}

	// Parent is not optional
	switch depType {
	case "optional":
		return fmt.Sprintf("optional_%s", parentFolder), nil
	case "required":
		return parentFolder, nil
	default:
		return "", fmt.Errorf("unknown dependency type: %s", depType)
	}
}

// installDependencies recursively installs all dependencies
func installDependencies(sc scope.Scope, deps []dependency, parentFolder string) error {
	for _, dep := range deps {
		depFolder, err := getDependencyFolder(parentFolder, dep.DependencyType)
		if err != nil {
			return err
		}

		sc.Logf("Installing dependency %s (%s) in %s",
			dep.ProjectID, dep.DependencyType, depFolder)

		if err := AddProjectByID(sc, dep.ProjectID, depFolder); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", dep.ProjectID, err)
		}
	}
	return nil
}

//# --- Main Public Functions ---

// AddProjectByID downloads and installs a mod/plugin from Modrinth by project ID
func AddProjectByID(sc scope.Scope, projectID string, folder string) error {
	// 1. Get project info
	info, err := getProjectInfo(projectID)
	if err != nil {
		return err
	}

	// 2. Check if already installed
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	if isProjectInstalled(sc, info.Slug, folder, osmiumConf) {
		sc.Logf("%s already installed", info.Slug)
		return nil
	}

	// 3. Get compatible versions
	versions, err := getProjectVersions(info.Slug, osmiumConf)
	if err != nil {
		return err
	}

	// 4. Download the latest version
	latestVersion := versions[0]
	if len(latestVersion.Files) == 0 {
		return fmt.Errorf("no files found in the latest version")
	}

	fileInfo := latestVersion.Files[0]
	sc.Logf("Downloading %s...", fileInfo.Filename)

	if err := downloadFile(sc, fileInfo.URL, folder, fileInfo.Filename); err != nil {
		return err
	}

	// 5. Update config
	if err := updateConfigWithProject(sc, info.Slug, folder, latestVersion, osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	// 6. Install dependencies
	if err := installDependencies(sc, latestVersion.Dependencies, folder); err != nil {
		return err
	}

	return nil
}

func RemoveProjectByID(sc scope.Scope, projectID string, folder string) error {
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	var project config.Project
	var ok bool

	switch folder {
	case "mods":
		if project, ok = osmiumConf.Mods[projectID]; !ok {
			return fmt.Errorf("project %s is not installed", projectID)
		}
		delete(osmiumConf.Mods, projectID)
	case "plugins":
		if project, ok = osmiumConf.Plugins[projectID]; !ok {
			return fmt.Errorf("project %s is not installed", projectID)
		}
		delete(osmiumConf.Plugins, projectID)
	default: // optional_mods or optional_plugins
	}

	filePath := sc.Path(folder, project.FileName)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove project file: %w", err)
	}

	if err := config.WriteConfigIn(sc.Dir, osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	return nil
}

// calculateSHA1 calculates the SHA1 checksum of a file
func calculateSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func getProjectByHash(hash string) (modrinthVersion, error) {
	projectUrl := fmt.Sprintf("https://api.modrinth.com/v2/version_file/%s", hash)

	resp, err := doModrinthRequest(projectUrl)
	if err != nil {
		return modrinthVersion{}, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return modrinthVersion{}, fmt.Errorf("failed to fetch version by hash: %s", resp.Status)
	}

	var project modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return modrinthVersion{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return project, nil
}

// processDirectory reads a directory and calculates SHA1 for each file
func processDirectory(sc scope.Scope, folder string) error {
	if _, err := os.Stat(sc.Path(folder)); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(sc.Path(folder))
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", folder, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // skip subdirectories
		}

		filePath := sc.Path(folder, entry.Name())

		// filter .jar files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			continue
		}

		checksum, err := calculateSHA1(filePath)
		if err != nil {
			sc.Logf("failed to calculate checksum for %s: %v", filePath, err)
			continue
		}

		project, err := getProjectByHash(checksum)
		if err != nil {
			sc.Logf("failed to fetch project metadata for %s: %v", filePath, err)
			continue
		}

		// 1. Get project info
		info, err := getProjectInfo(project.ProjectID)
		if err != nil {
			sc.Logf("%v", err)
			continue
		}

		// 2. Check if already installed
		osmiumConf, err := config.ReadConfigIn(sc.Dir)
		if err != nil {
			return fmt.Errorf("failed to read osmium.json: %w", err)
		}

		if isProjectInstalled(sc, info.Slug, folder, osmiumConf) {
			sc.Logf("%s already in osmium.json", info.Slug)
			continue
		}

		// 3. Update config
		if len(project.Files) == 0 {
			sc.Logf("no files found in the latest version for %s", info.Slug)
			continue
		}

		if err := updateConfigWithProject(sc, info.Slug, folder, project, osmiumConf); err != nil {
			return fmt.Errorf("failed to update osmium.json: %w", err)
		}

		// // 6. Install dependencies
		// if err := installDependencies(project.Dependencies, folder); err != nil {
		// 	return err
		// }

		sc.Logf("Tracked %s", info.Slug)
	}

	return nil
}

// trackProjects reads the plugins and mods directories and calculates SHA
func TrackProjects(sc scope.Scope) error {
	// Process plugins directory
	if err := processDirectory(sc, "plugins"); err != nil {
		return err
	}

	// Process mods directory
	if err := processDirectory(sc, "mods"); err != nil {
		return err
	}

	return nil
}

func UpdateProject(sc scope.Scope, projectID string, folder string) error {
	// 1. Check if already installed
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	if !isProjectInstalled(sc, projectID, folder, osmiumConf) {
		return fmt.Errorf("%s is not installed", projectID)
	}

	// 3. Get compatible versions
	versions, err := getProjectVersions(projectID, osmiumConf)
	if err != nil {
		return err
	}

	// 4. Download the latest version
	latestVersion := versions[0]
	if len(latestVersion.Files) == 0 {
		return fmt.Errorf("no files found in the latest version")
	}

	fileInfo := latestVersion.Files[0]

	var currentProject config.Project
	switch folder {
	case "mods":
		currentProject = osmiumConf.Mods[projectID]
	case "plugins":
		currentProject = osmiumConf.Plugins[projectID]
		// optional folders don't get tracked in config
	}

	if currentProject.SHA1 == fileInfo.Hashes.SHA1 {
		sc.Logf("%s is the latest version", projectID)
		return nil
	}

	sc.Logf("Updating %s...", fileInfo.Filename)

	os.Remove(sc.Path(folder, currentProject.FileName))

	if err := downloadFile(sc, fileInfo.URL, folder, fileInfo.Filename); err != nil {
		return err
	}

	// 5. Update config
	if err := updateConfigWithProject(sc, projectID, folder, latestVersion, osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	// 6. Install dependencies
	if err := installDependencies(sc, latestVersion.Dependencies, folder); err != nil {
		return err
	}

	return nil
}

func UpdateAllProjects(sc scope.Scope, folder string) error {
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	switch folder {
	case "mods":
		for mod := range osmiumConf.Mods {
			if err := UpdateProject(sc, mod, "mods"); err != nil {
				sc.Logf("%v", err)
			}
		}
	case "plugins":
		for plugin := range osmiumConf.Plugins {
			if err := UpdateProject(sc, plugin, "plugins"); err != nil {
				sc.Logf("%v", err)
			}
		}
	case "all":
		for mod := range osmiumConf.Mods {
			if err := UpdateProject(sc, mod, "mods"); err != nil {
				sc.Logf("%v", err)
			}
		}
		for plugin := range osmiumConf.Plugins {
			if err := UpdateProject(sc, plugin, "plugins"); err != nil {
				sc.Logf("%v", err)
			}
		}
	default:
		return fmt.Errorf("Invalid case")
	}

	return nil
}

func InstallProjectByID(sc scope.Scope, projectID string, folder string) error {
	// 1. Check if already installed
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	if isProjectInstalled(sc, projectID, folder, osmiumConf) {
		sc.Logf("%s already installed", projectID)
		return nil
	}

	// 3. Get compatible versions
	versions, err := getProjectVersions(projectID, osmiumConf)
	if err != nil {
		return err
	}

	// 4. Download the latest version
	latestVersion := versions[0]
	if len(latestVersion.Files) == 0 {
		return fmt.Errorf("no files found in the latest version")
	}

	fileInfo := latestVersion.Files[0]
	sc.Logf("Downloading %s...", fileInfo.Filename)

	if err := downloadFile(sc, fileInfo.URL, folder, fileInfo.Filename); err != nil {
		return err
	}

	// 5. Install dependencies
	if err := installDependencies(sc, latestVersion.Dependencies, folder); err != nil {
		return err
	}

	return nil
}

func InstallProjectsFromConfig(sc scope.Scope) error {
	osmiumConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	for mod := range osmiumConf.Mods {
		if err := InstallProjectByID(sc, mod, "mods"); err != nil {
			sc.Logf("%v", err)
		}
	}
	for plugin := range osmiumConf.Plugins {
		if err := InstallProjectByID(sc, plugin, "plugins"); err != nil {
			sc.Logf("%v", err)
		}
	}

	return nil
}

// migrateProject attempts to migrate a single project to new loader/version
// Returns true if migrated successfully, false if incompatible (needs backup)
func migrateProject(sc scope.Scope, slug string, folder string, newConf *config.OsmiumConfig) (bool, error) {
	// Get compatible versions for the new loader/version
	versions, err := getProjectVersions(slug, newConf)
	if err != nil {
		// Project not compatible with new loader/version
		return false, err
	}

	if len(versions) == 0 {
		return false, fmt.Errorf("no compatible versions found")
	}

	// Get current project to delete old file
	oldConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return false, err
	}

	var oldProject config.Project
	var exists bool

	switch folder {
	case "mods", "optional_mods":
		oldProject, exists = oldConf.Mods[slug]
	case "plugins", "optional_plugins":
		oldProject, exists = oldConf.Plugins[slug]
	}

	// Download new version
	latestVersion := versions[0]
	if len(latestVersion.Files) == 0 {
		return false, fmt.Errorf("no files found in the latest version")
	}

	fileInfo := latestVersion.Files[0]
	sc.Logf("Migrating %s to %s...", slug, fileInfo.Filename)

	if err := downloadFile(sc, fileInfo.URL, folder, fileInfo.Filename); err != nil {
		return false, err
	}

	// Remove old file if it exists and is different
	if exists && oldProject.FileName != fileInfo.Filename {
		oldPath := sc.Path(folder, oldProject.FileName)
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			sc.Logf("Warning: failed to remove old file %s: %v", oldPath, err)
		}
	}

	// Update config (only for tracked folders)
	if folder == "mods" || folder == "plugins" {
		if err := updateConfigWithProject(sc, slug, folder, latestVersion, newConf); err != nil {
			return false, err
		}
	}

	// Install dependencies
	if err := installDependencies(sc, latestVersion.Dependencies, folder); err != nil {
		sc.Logf("Warning: failed to install dependencies for %s: %v", slug, err)
	}

	return true, nil
}

// processProjectsForMigration processes all projects in a folder for migration
func processProjectsForMigration(sc scope.Scope, folder string, projects map[string]config.Project, newConf *config.OsmiumConfig) error {
	backupFolder := sc.Path("migration_backup", folder)

	for slug := range projects {
		migrated, err := migrateProject(sc, slug, folder, newConf)
		if !migrated {
			// Project is not compatible, move to backup
			sc.Logf("⚠ Warning: %s is not available for %s %s", slug, newConf.Loader, newConf.Version)
			sc.Logf("Moving to backup folder: %s", backupFolder)

			// Create backup folder
			if err := os.MkdirAll(backupFolder, 0755); err != nil {
				return fmt.Errorf("failed to create backup folder: %w", err)
			}

			// Get the project to find its filename
			oldConf, err := config.ReadConfigIn(sc.Dir)
			if err != nil {
				return err
			}

			var project config.Project
			if folder == "mods" || folder == "optional_mods" {
				project = oldConf.Mods[slug]
			} else {
				project = oldConf.Plugins[slug]
			}

			// Move file to backup
			srcPath := sc.Path(folder, project.FileName)
			dstPath := filepath.Join(backupFolder, project.FileName)

			if err := os.Rename(srcPath, dstPath); err != nil && !os.IsNotExist(err) {
				sc.Logf("Warning: failed to move %s to backup: %v", srcPath, err)
			}

			// Remove from config
			switch folder {
			case "mods":
				delete(newConf.Mods, slug)
			case "plugins":
				delete(newConf.Plugins, slug)
			}
		} else if err != nil {
			sc.Logf("Warning: error migrating %s: %v", slug, err)
		}
	}

	return nil
}

func MigrateServer(sc scope.Scope, loader string, version string) error {
	sc.Logf("\n=== Starting server migration to %s %s ===", loader, version)

	// Read current config
	oldConf, err := config.ReadConfigIn(sc.Dir)
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	sc.Logf("Current: %s %s", oldConf.Loader, oldConf.Version)
	sc.Logf("Target:  %s %s", loader, version)

	// Create new config with updated loader and version
	newConf := &config.OsmiumConfig{
		Category: oldConf.Category,
		Loader:   loader,
		Version:  version,
		// The allocation is the server's, not the loader's: migrating must not
		// quietly reset it and leave the JVM without a heap size.
		AllocatedRAM: oldConf.AllocatedRAM,
		Mods:         make(map[string]config.Project),
		Plugins:      make(map[string]config.Project),
	}

	// Copy existing mods and plugins to new config (will be updated during migration)
	for k, v := range oldConf.Mods {
		newConf.Mods[k] = v
	}
	for k, v := range oldConf.Plugins {
		newConf.Plugins[k] = v
	}

	// Step 1: Migrate mods (if any)
	if len(oldConf.Mods) > 0 {
		sc.Logf("--- Migrating mods ---")
		if err := processProjectsForMigration(sc, "mods", oldConf.Mods, newConf); err != nil {
			return err
		}

		// Process optional_mods if the folder exists
		if _, err := os.Stat(sc.Path("optional_mods")); err == nil {
			sc.Logf("--- Migrating optional mods ---")
			if err := processProjectsForMigration(sc, "optional_mods", oldConf.Mods, newConf); err != nil {
				return err
			}
		}
	}

	// Step 2: Migrate plugins (if any)
	if len(oldConf.Plugins) > 0 {
		sc.Logf("--- Migrating plugins ---")
		if err := processProjectsForMigration(sc, "plugins", oldConf.Plugins, newConf); err != nil {
			return err
		}

		// Process optional_plugins if the folder exists
		if _, err := os.Stat(sc.Path("optional_plugins")); err == nil {
			sc.Logf("--- Migrating optional plugins ---")
			if err := processProjectsForMigration(sc, "optional_plugins", oldConf.Plugins, newConf); err != nil {
				return err
			}
		}
	}

	// Step 3: Download and install new server.jar (only if loader or version changed)
	//
	// The jar comes before the manifest is rewritten: a failed download would
	// otherwise leave osmium.json claiming a version the folder does not have,
	// and everything that reads the manifest — the server list included —
	// would report the server as something it is not.
	if strings.EqualFold(oldConf.Loader, loader) && oldConf.Version == version {
		sc.Logf("--- Server.jar unchanged ---")
		sc.Logf("✓ Same loader and version, skipping server.jar download")
	} else {
		sc.Logf("--- Installing new server.jar ---")
		if err := util.DownloadJarInto(sc, loader, version); err != nil {
			return fmt.Errorf("failed to download new server.jar: %w", err)
		}
		sc.Logf("✓ Server files downloaded")
	}

	// Step 4: Update config file with new loader and version
	sc.Logf("--- Updating osmium.json ---")
	if err := config.WriteConfigIn(sc.Dir, newConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}
	sc.Logf("✓ Config updated")

	sc.Logf("\n=== Migration completed! ===")
	sc.Logf("Server migrated from %s %s to %s %s", oldConf.Loader, oldConf.Version, loader, version)
	sc.Logf("Note: World folders, server.properties, and other config files remain intact.")
	if _, err := os.Stat(sc.Path("migration_backup")); err == nil {
		sc.Logf("Incompatible mods/plugins have been moved to 'migration_backup' folder.")
	}

	return nil
}
