package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/limelamp/osmium/internal/config"
)

//* API didn't work as expected
// type modrinthProject struct {
// 	ProjectType string `json:"project_type"`
// }

type slugData struct {
	Slug string `json:"slug"`
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

// works for plugins and mods
func AddProjectByID(projectID string, folder string) error {
	//* To be considered later
	// // 1. Define if the project is a mod or plugin
	// projectUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s", projectID)
	// projectResp, err := http.Get(projectUrl)
	// if err != nil {
	// 	return fmt.Errorf("failed to fetch the project: %w", err)
	// }
	// defer projectResp.Body.Close()

	// var project modrinthProject
	// if err := json.NewDecoder(projectResp.Body).Decode(&project); err != nil {
	// 	return fmt.Errorf("failed to decode project response: %w", err)
	// }

	// fmt.Println(project.ProjectType)
	//* To be considered later

	//Todo: use this url to identify if the project is a mod or plugin or hybrid "loaders": [...]
	// 1. Retrieve the slug instead of ID for osmium.json (USE TO IDENTIFY PROJECT_TYPE LATER)
	slugUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s", projectID)

	client := &http.Client{}
	slugReq, _ := http.NewRequest("GET", slugUrl, nil)
	slugReq.Header.Set("User-Agent", "Osmium-Manager/1.0") // Modrinth REQUIRES this

	slugResp, err := client.Do(slugReq)
	if err != nil {
		return fmt.Errorf("failed to fetch slug: %w", err)
	}
	defer slugResp.Body.Close()

	var slug slugData
	if err := json.NewDecoder(slugResp.Body).Decode(&slug); err != nil {
		return fmt.Errorf("failed to decode slug response: %w", err)
	}

	// check if the file already exists
	osmiumConf, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read osmium.json: %w", err)
	}

	switch folder {
	case "mods", "optional_mods":
		if _, ok := osmiumConf.Mods[slug.Slug]; ok {
			fmt.Printf("%s already installed\n", slug.Slug)
			return nil
		}
	case "plugins", "optional_plugins":
		if _, ok := osmiumConf.Plugins[slug.Slug]; ok {
			fmt.Printf("%s already installed\n", slug.Slug)
			return nil
		}
	default: // optional_mods or optional_plugins
	}

	// 2. Finding projectURL by slug
	baseUrl := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version", slug.Slug)

	// parsing the url and queries
	projectUrl, err := url.Parse(baseUrl)
	if err != nil {
		return fmt.Errorf("failed to parse base url: %w", err)
	}
	q := projectUrl.Query()
	q.Set("game_versions", fmt.Sprintf(`["%s"]`, osmiumConf.Version))
	q.Set("loaders", fmt.Sprintf(`["%s"]`, strings.ToLower(osmiumConf.Loader)))
	projectUrl.RawQuery = q.Encode()

	// fmt.Println(projectUrl) //* Keep for debugging

	req, _ := http.NewRequest("GET", projectUrl.String(), nil)
	req.Header.Set("User-Agent", "Osmium-Manager/1.0") // Modrinth REQUIRES this

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	var versions []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("no compatible versions found for Minecraft %s", osmiumConf.Version)
	}

	// 3. Pick the first file from the newest version
	if len(versions[0].Files) == 0 {
		return fmt.Errorf("no files found in the latest version")
	}
	fileInfo := versions[0].Files[0]
	fmt.Printf("Downloading %s...\n\n", fileInfo.Filename)

	// 4. Download the actual .jar
	fileResp, err := http.Get(fileInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer fileResp.Body.Close()

	// Ensure the plugins folder exists
	if err := os.MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("failed to create %s folder: %w", folder, err)
	}

	out, err := os.Create(folder + "/" + fileInfo.Filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, fileResp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 5. Adding project data to Osmium.json
	switch folder {
	case "mods":
		osmiumConf.Mods[slug.Slug] = config.Project{
			VersionNumber: versions[0].VersionNumber,
			FileName:      fileInfo.Filename,
			DownloadURL:   fileInfo.URL,
		}
	case "plugins":
		osmiumConf.Plugins[slug.Slug] = config.Project{
			VersionNumber: versions[0].VersionNumber,
			FileName:      fileInfo.Filename,
			DownloadURL:   fileInfo.URL,
		}
	default: // optional_mods or optional_plugins
	}

	if err := config.WriteConfig(osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	// 6. Check for dependencies
	deps := versions[0].Dependencies

	// Run the dependency install loop
	for _, dep := range deps {
		depFolder := ""
		switch dep.DependencyType {
		case "optional":
			depFolder = fmt.Sprintf("optional_%s", folder)
		case "required":
			// if the depFolder in recursive function is "optional_mods", required projects of optional projects are installed in optional directory lol
			depFolder = folder
		default:
			return fmt.Errorf("unknown dependency type %s", dep.ProjectID)
		}

		fmt.Printf("Installing dependency %s (%s) in %s\n", dep.ProjectID, dep.DependencyType, depFolder)

		// recursively download a dependency (installs required dependencies of optional dependencies to optional directory smh)
		if err := AddProjectByID(dep.ProjectID, depFolder); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", dep.ProjectID, err)
		}
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

	if err := os.Remove(fmt.Sprintf("%s/%s", folder, project.FileName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove project file: %w", err)
	}

	if err := config.WriteConfig(osmiumConf); err != nil {
		return fmt.Errorf("failed to update osmium.json: %w", err)
	}

	return nil
}
