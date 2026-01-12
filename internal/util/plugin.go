package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// The structure of the Modrinth Version API response
type modrinthVersion struct {
	Files []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
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

// Black magic
func DownloadPluginByID(projectID string) error {
	mcVersion := getInstalledVersion()
	client := &http.Client{}

	// 1. Get the list of versions for this specific project
	// Filter by game_versions to make sure we get the right one
	url := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version?game_versions=[\"%s\"]", projectID, mcVersion)
	//https://api.modrinth.com/v2/project/skinrestorer/version?game_versions="1.21.11"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Osmium-Manager/1.0") // Modrinth REQUIRES this

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var versions []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return err
	}

	if len(versions) == 0 {
		return fmt.Errorf("no compatible versions found for %s", mcVersion)
	}

	// 2. Pick the first file from the newest version
	fileInfo := versions[0].Files[0]
	fmt.Printf("Downloading %s...\n", fileInfo.Filename)

	// 3. Download the actual .jar
	fileResp, err := http.Get(fileInfo.URL)
	if err != nil {
		return err
	}
	defer fileResp.Body.Close()

	// Ensure the plugins folder exists
	os.MkdirAll("plugins", 0755)

	out, err := os.Create("plugins/" + fileInfo.Filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, fileResp.Body)
	return err
}
