package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/constants"
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
	ID            string   `json:"id"`
	VersionNumber string   `json:"version_number"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	VersionType   string   `json:"version_type"` // "release", "beta", "alpha"
	Files         []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
	} `json:"files"`
	Dependencies []dependency `json:"dependencies"`
}

//# --- HTTP Client Helpers ---

// createModrinthClient returns an HTTP client configured for Modrinth API
func createModrinthClient() *http.Client {
	return &http.Client{}
}

// doModrinthRequest performs an HTTP request with required Modrinth headers
func doModrinthRequest(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Osmium-Manager/1.0")
	return client.Do(req)
}

//# --- project functions ---

// getProjectInfo fetches the project info (slug and loaders) from Modrinth API by project ID
func getProjectInfo(projectID string) (projectInfo, error) {
	slugUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s", projectID)

	client := createModrinthClient()
	resp, err := doModrinthRequest(client, slugUrl)
	if err != nil {
		return projectInfo{}, fmt.Errorf("failed to fetch slug: %w", err)
	}
	defer resp.Body.Close()

	var info projectInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return projectInfo{}, fmt.Errorf("failed to decode slug response: %w", err)
	}

	return info, nil
}

// isProjectInstalled checks if a project is already installed in the config
func isProjectInstalled(slug, folder string, conf *config.OsmiumConfig) bool {
	switch folder {
	case "mods", "optional_mods":
		_, ok := conf.Mods[slug]
		return ok
	case "plugins", "optional_plugins":
		_, ok := conf.Plugins[slug]
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
	fmt.Println(loaders)
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

	client := createModrinthClient()
	resp, err := doModrinthRequest(client, projectUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

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
func downloadFile(url, folder, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Ensure folder exists
	if err := os.MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("failed to create %s folder: %w", folder, err)
	}

	filePath := filepath.Join(folder, filename)
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// updateConfigWithProject adds the project to the appropriate section in config
func updateConfigWithProject(slug, folder string, version modrinthVersion, conf *config.OsmiumConfig) error {
	if len(version.Files) == 0 {
		return fmt.Errorf("no files found in version")
	}

	fileInfo := version.Files[0]
	project := config.Project{
		VersionNumber: version.VersionNumber,
		FileName:      fileInfo.Filename,
		DownloadURL:   fileInfo.URL,
	}

	switch folder {
	case "mods":
		conf.Mods[slug] = project
	case "plugins":
		conf.Plugins[slug] = project
		// optional folders don't get tracked in config
	}

	return config.WriteConfig(conf)
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
func installDependencies(deps []dependency, parentFolder string) error {
	for _, dep := range deps {
		depFolder, err := getDependencyFolder(parentFolder, dep.DependencyType)
		if err != nil {
			return err
		}

		fmt.Printf("Installing dependency %s (%s) in %s\n",
			dep.ProjectID, dep.DependencyType, depFolder)

		if err := AddProjectByID(dep.ProjectID, depFolder); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", dep.ProjectID, err)
		}
	}
	return nil
}

//# --- Main Public Functions ---

// AddProjectByID downloads and installs a mod/plugin from Modrinth by project ID
func AddProjectByID(projectID string, folder string) error {
	// 1. Get project info
	info, err := getProjectInfo(projectID)
	if err != nil {
		return err
	}

	// 2. Check if already installed
	osmiumConf, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	if isProjectInstalled(info.Slug, folder, osmiumConf) {
		fmt.Printf("%s already installed\n\n", info.Slug)
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
	fmt.Printf("Downloading %s...\n\n", fileInfo.Filename)

	if err := downloadFile(fileInfo.URL, folder, fileInfo.Filename); err != nil {
		return err
	}

	// 5. Update config
	if err := updateConfigWithProject(info.Slug, folder, latestVersion, osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	// 6. Install dependencies
	if err := installDependencies(latestVersion.Dependencies, folder); err != nil {
		return err
	}

	return nil
}

func RemoveProjectByID(projectID string, folder string) error {
	osmiumConf, err := config.ReadConfig()
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

	filePath := filepath.Join(folder, project.FileName)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove project file: %w", err)
	}

	if err := config.WriteConfig(osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	return nil
}
