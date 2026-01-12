package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Json structs ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// The "Master Manifest" lists all versions
type vanillaVersionManifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID  string `json:"id"`  // e.g., "1.21.1"
		URL string `json:"url"` // The link to this version's specific JSON
	} `json:"versions"`
}

// The "Version Specific JSON" contains the actual jar link
type vanillaVersionPackage struct {
	Downloads struct {
		Server struct {
			URL string `json:"url"`
		} `json:"server"`
	} `json:"downloads"`
}

// https://api.papermc.io/v2/projects/paper/versions/{version}/builds/{build}/downloads/{filename}
type paperBuilds struct {
	Builds []int `json:"builds"` // Build numbers
}

type paperDownloads struct {
	Downloads struct {
		Application struct {
			Name string `json:"name"`
		} `json:"application"`
	} `json:"downloads"`
}

// General functions ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
func getVanillaServerURL(targetVersion string) (string, error) {
	// Step 1: Get the Master Manifest
	resp, err := http.Get("https://launchermeta.mojang.com/mc/game/version_manifest.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var manifest vanillaVersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", err
	}

	// Step 2: Find the specific version in the list
	var versionDetailURL string
	for _, v := range manifest.Versions {
		if v.ID == targetVersion {
			versionDetailURL = v.URL
			break
		}
	}

	if versionDetailURL == "" {
		return "", fmt.Errorf("version %s not found", targetVersion)
	}

	// Step 3: Get the specific version's JSON package
	resp, err = http.Get(versionDetailURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pkg vanillaVersionPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", err
	}

	return pkg.Downloads.Server.URL, nil
}

func getPaperServerURL(version string) (string, error) {
	// Step 1: Get the builds list to find the LATEST build number
	buildsURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s", version)
	resp, err := http.Get(buildsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var buildsData paperBuilds
	if err := json.NewDecoder(resp.Body).Decode(&buildsData); err != nil {
		return "", err
	}

	if len(buildsData.Builds) == 0 {
		return "", fmt.Errorf("no builds found for version %s", version)
	}
	latestBuild := buildsData.Builds[len(buildsData.Builds)-1]

	// Step 2: Get the filename for that specific build
	infoURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d", version, latestBuild)
	resp, err = http.Get(infoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info paperDownloads
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	filename := info.Downloads.Application.Name

	// Step 3: Construct the final direct download link
	finalURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d/downloads/%s",
		version, latestBuild, filename)

	// Potential shortcut method
	// finalURL := "paper-" + targetVersion + "-" + strconv.Itoa(latestBuild) + ".jar"
	// fmt.Println(finalURL)
	// return finalURL, nil

	return finalURL, nil
}

func downloadFile(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func DownloadJar(jarType string, jarVersion string) error {
	// Deciding which url
	url := ""
	var err error
	switch jarType {
	case "Vanilla":
		url, err = getVanillaServerURL(jarVersion)
		if err != nil {
			return err
		}
	// case "Bukkit":
	// 	url = ""
	// case "Spigot":
	// 	url = ""
	case "Paper":
		url, _ = getPaperServerURL(jarVersion)
	case "Purpur":
		url = "https://api.purpurmc.org/v2/purpur/" + jarVersion + "/latest/download"
	}

	output := "server.jar"

	fmt.Println("Downloading the required files....")

	if err := downloadFile(url, "server.jar"); err != nil {
		return err
	}

	fmt.Println("Download finished: ", output)
	return nil
}
