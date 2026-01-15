package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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

// Fabric API structs
type fabricInstallerVersion struct {
	URL     string `json:"url"`
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type fabricLoaderVersion struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// NeoForge version mapping (MC version to NeoForge version prefix)
var NeoForgeVersionMap = map[string]string{
	"1.21.11": "21.11",
	"1.21.4":  "21.4",
	"1.21.1":  "21.1",
}

// Youer/Mohist API structs
type mohistBuildResponse struct {
	Number    int    `json:"number"`
	ForgeVer  string `json:"forgeVersion"`
	FileMd5   string `json:"fileMd5"`
	CreatedAt int64  `json:"createdAt"`
}

// Note: modrinthVersion struct is defined in plugin.go and shared across the package

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

func getFabricServerURL(mcVersion string) (string, error) {
	// Step 1: Get the latest stable loader version
	loaderResp, err := http.Get("https://meta.fabricmc.net/v2/versions/loader")
	if err != nil {
		return "", err
	}
	defer loaderResp.Body.Close()

	var loaders []fabricLoaderVersion
	if err := json.NewDecoder(loaderResp.Body).Decode(&loaders); err != nil {
		return "", err
	}

	// Find the first stable loader
	var loaderVersion string
	for _, l := range loaders {
		if l.Stable {
			loaderVersion = l.Version
			break
		}
	}
	if loaderVersion == "" && len(loaders) > 0 {
		loaderVersion = loaders[0].Version // fallback to first
	}

	// Step 2: Get the latest stable installer version
	installerResp, err := http.Get("https://meta.fabricmc.net/v2/versions/installer")
	if err != nil {
		return "", err
	}
	defer installerResp.Body.Close()

	var installers []fabricInstallerVersion
	if err := json.NewDecoder(installerResp.Body).Decode(&installers); err != nil {
		return "", err
	}

	var installerVersion string
	for _, i := range installers {
		if i.Stable {
			installerVersion = i.Version
			break
		}
	}
	if installerVersion == "" && len(installers) > 0 {
		installerVersion = installers[0].Version
	}

	// Step 3: Construct direct server JAR download URL
	// Format: https://meta.fabricmc.net/v2/versions/loader/{mcVersion}/{loaderVersion}/{installerVersion}/server/jar
	url := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/%s/server/jar",
		mcVersion, loaderVersion, installerVersion)

	return url, nil
}

// getNeoForgeInstallerURL returns the installer JAR URL for NeoForge
func getNeoForgeInstallerURL(mcVersion string) (string, string, error) {
	// Get the NeoForge version prefix for this MC version
	neoforgePrefix, ok := NeoForgeVersionMap[mcVersion]
	if !ok {
		return "", "", fmt.Errorf("NeoForge version not found for Minecraft %s", mcVersion)
	}

	// We'll use a known stable version for each MC version
	// These are hardcoded stable versions that work
	stableVersions := map[string]string{
		"1.21.11": "21.11.25-beta", // Latest for 1.21.11
		"1.21.4":  "21.4.156",      // Stable for 1.21.4
		"1.21.1":  "21.1.216",      // Stable for 1.21.1
	}

	neoforgeVersion, ok := stableVersions[mcVersion]
	if !ok {
		return "", "", fmt.Errorf("no stable NeoForge version configured for Minecraft %s", mcVersion)
	}

	// Construct installer URL
	// Format: https://maven.neoforged.net/releases/net/neoforged/neoforge/{version}/neoforge-{version}-installer.jar
	url := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar",
		neoforgeVersion, neoforgeVersion)

	_ = neoforgePrefix // silence unused variable

	return url, neoforgeVersion, nil
}

// getYouerServerURL returns the direct download URL for Youer (NeoForge + Paper hybrid)
func getYouerServerURL(mcVersion string) (string, error) {
	// Youer API: https://mohistmc.com/api/v2/projects/youer/versions/{version}/builds/latest/download
	url := fmt.Sprintf("https://mohistmc.com/api/v2/projects/youer/versions/%s/builds/latest/download", mcVersion)
	return url, nil
}

// getFabricAPIURL fetches the latest Fabric API version for a specific Minecraft version from Modrinth
func getFabricAPIURL(mcVersion string) (string, string, error) {
	// Create HTTP client with proper User-Agent (required by Modrinth API)
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.modrinth.com/v2/project/fabric-api/version", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Modrinth API error: %s", resp.Status)
	}

	var versions []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", "", err
	}

	// Find the first release version that matches our MC version and is for fabric loader
	for _, v := range versions {
		// Check if this version supports our MC version
		supportsVersion := false
		for _, gv := range v.GameVersions {
			if gv == mcVersion {
				supportsVersion = true
				break
			}
		}

		if !supportsVersion {
			continue
		}

		// Check if it's for fabric loader
		supportsFabric := false
		for _, loader := range v.Loaders {
			if loader == "fabric" {
				supportsFabric = true
				break
			}
		}

		if !supportsFabric {
			continue
		}

		// Prefer release versions, but accept beta if no release found
		if v.VersionType == "release" || v.VersionType == "beta" {
			// Find the primary file
			for _, file := range v.Files {
				if file.Primary {
					return file.URL, file.Filename, nil
				}
			}
			// If no primary file, use the first one
			if len(v.Files) > 0 {
				return v.Files[0].URL, v.Files[0].Filename, nil
			}
		}
	}

	return "", "", fmt.Errorf("no Fabric API version found for Minecraft %s", mcVersion)
}

// DownloadFabricAPI downloads the Fabric API mod to the mods folder
func DownloadFabricAPI(mcVersion string) error {
	// Create mods folder if it doesn't exist
	if err := os.MkdirAll("mods", 0755); err != nil {
		return fmt.Errorf("failed to create mods folder: %w", err)
	}

	// Get Fabric API URL
	url, filename, err := getFabricAPIURL(mcVersion)
	if err != nil {
		return err
	}

	fmt.Println("Downloading Fabric API...")

	// Download with proper User-Agent
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Create the file in mods folder
	filepath := "mods/" + filename
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Fabric API downloaded: %s\n", filepath)
	return nil
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
	installerFilename := ""
	isInstaller := false

	switch jarType {
	case "Vanilla":
		url, err = getVanillaServerURL(jarVersion)
		if err != nil {
			return err
		}
	case "Paper":
		url, err = getPaperServerURL(jarVersion)
		if err != nil {
			return err
		}
	case "Purpur":
		url = "https://api.purpurmc.org/v2/purpur/" + jarVersion + "/latest/download"
	case "Fabric":
		url, err = getFabricServerURL(jarVersion)
		if err != nil {
			return err
		}
	case "NeoForge":
		var neoforgeVer string
		url, neoforgeVer, err = getNeoForgeInstallerURL(jarVersion)
		if err != nil {
			return err
		}
		installerFilename = fmt.Sprintf("neoforge-%s-installer.jar", neoforgeVer)
		isInstaller = true
	case "Youer":
		url, err = getYouerServerURL(jarVersion)
		if err != nil {
			return err
		}
	}

	// Determine output filename
	output := "server.jar"
	if isInstaller {
		output = installerFilename
	}

	fmt.Println("Downloading the required files....")

	if err := downloadFile(url, output); err != nil {
		return err
	}

	fmt.Println("Download finished: ", output)

	// If it's an installer (NeoForge), run the installer
	if isInstaller {
		fmt.Println("Running installer...")
		if err := RunModLoaderInstaller(jarType, output); err != nil {
			return err
		}
	}

	// For Fabric, automatically download Fabric API as it's required by most mods
	if jarType == "Fabric" {
		fmt.Println("\nFabric API is required by most Fabric mods. Downloading automatically...")
		if err := DownloadFabricAPI(jarVersion); err != nil {
			fmt.Printf("Warning: Could not download Fabric API: %v\n", err)
			fmt.Println("You may need to download it manually from https://modrinth.com/mod/fabric-api")
		}
	}

	return nil
}

// RunModLoaderInstaller runs the mod loader installer to set up the server
func RunModLoaderInstaller(loaderType string, installerFile string) error {
	var cmd *exec.Cmd

	switch loaderType {
	case "NeoForge":
		// java -jar neoforge-installer.jar --installServer
		cmd = exec.Command("java", "-jar", installerFile, "--installServer")
	default:
		return fmt.Errorf("unknown mod loader type: %s", loaderType)
	}

	// Run in current directory
	cmd.Dir, _ = os.Getwd()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Installing %s server...\n", loaderType)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installer failed: %w", err)
	}

	fmt.Println("Installation complete!")
	return nil
}

// IsModLoader returns true if the jar type requires special handling (installer-based)
func IsModLoader(jarType string) bool {
	switch jarType {
	case "NeoForge":
		return true
	default:
		return false
	}
}

// GetServerRunCommand returns the command needed to run the server for a given jar type
func GetServerRunCommand(jarType string) (string, []string) {
	// f, err := os.Create("output.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()

	// // Write jarType to file
	// fmt.Fprintln(f, jarType)

	switch jarType {
	case "NeoForge":
		// NeoForge creates run.bat/run.sh after installation - use those directly
		switch runtime.GOOS {
		case "windows": // On Windows, run the batch file
			return "cmd", []string{"/c", "run.bat", "nogui"}
		case "linux":
			return "bash", []string{"run.sh", "nogui"}
		case "darwin":
			return "sh", []string{"run.sh", "nogui"}
		case "freebsd":
			return "bash", []string{"run.sh", "nogui"}
		default:
			fmt.Println("Unsupported OS!")
			return " ", []string{""} // Something had to be returned
		}

	// case "Fabric":
	// 	// Fabric server jar is named fabric-server-launch.jar or server.jar
	// 	return "java", []string{"-jar", "-Xms4G", "server.jar", "nogui"}
	default:
		// Standard server.jar execution
		return "java", []string{"-jar", "-Xms4G", "server.jar", "nogui"}
	}
}
