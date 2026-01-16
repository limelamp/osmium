package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)


//* API didn't work as expected
// type modrinthProject struct {
// 	ProjectType string `json:"project_type"`
// }

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
}

func getInstalledVersion() string {
	// 1. Point to the "versions" folder relative to where the app is running
	path := "versions"

	// 2. Read all files/folders inside
	entries, err := os.ReadDir(path)
	if err != nil {
		return "" //, err
	}

	// 3. Loop through to find the first directory
	for _, entry := range entries {
		if entry.IsDir() {
			// Return the name of the folder (e.g., "1.21.11")
			return entry.Name() //, nil
		}
	}

	return ""
}

func DownloadPluginByID(projectID string) error {
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

	mcVersion := getInstalledVersion()
	client := &http.Client{}

	// 2. Get the list of versions for this specific project
	// Filter by game_versions and loaders to make sure we get the right one (loaders aren't implemented yet)
	url := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version?game_versions=[%s]", projectID, mcVersion)

	req, _ := http.NewRequest("GET", url, nil)
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
		return fmt.Errorf("no compatible versions found for Minecraft %s", mcVersion)
	}

	// 2. Pick the first file from the newest version
	if len(versions[0].Files) == 0 {
		return fmt.Errorf("no files found in the latest version")
	}
	fileInfo := versions[0].Files[0]
	fmt.Printf("Downloading %s...\n", fileInfo.Filename)

	// 3. Download the actual .jar
	fileResp, err := http.Get(fileInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer fileResp.Body.Close()

	// Ensure the plugins folder exists
	if err := os.MkdirAll("plugins", 0755); err != nil {
		return fmt.Errorf("failed to create plugins folder: %w", err)
	}

	out, err := os.Create("plugins/" + fileInfo.Filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, fileResp.Body)
	return err
}
