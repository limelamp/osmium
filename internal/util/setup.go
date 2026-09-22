// Package util downloads and installs the server software itself: the jars,
// the loader installers, and the command each of them is started with.
package util

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/scope"
)

// Json structs ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// The "Master Manifest" lists all versions
type vanillaVersionManifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID   string `json:"id"`   // e.g., "1.21.1"
		Type string `json:"type"` // release, snapshot etc.
		URL  string `json:"url"`  // The link to this version's specific JSON
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
// PaperMC's build list: https://fill.papermc.io/v3/projects/paper/versions/{version}/builds
//
// One entry per build, newest first, each carrying the download it produced.
type paperBuild struct {
	ID        int    `json:"id"`
	Channel   string `json:"channel"`
	Downloads map[string]struct {
		Name string `json:"name"`
		URL  string `json:"url"`
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

type neoForgeVersion struct {
	Versions []string `xml:"versioning>versions>version"`
}

type forgeVersion struct {
	Versions []string `xml:"versioning>versions>version"`
}

type quiltInstallerVersion struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

var setupHTTPClient = &http.Client{Timeout: 30 * time.Second}

func getWithStatus(url string) (*http.Response, error) {
	resp, err := setupHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	return resp, nil
}

// Note: modrinthVersion struct is defined in plugin.go and shared across the package

// GetMinecraftVersionsManifest fetches the manifest of every Minecraft version
func GetMinecraftVersionsManifest() (*vanillaVersionManifest, error) {
	resp, err := getWithStatus("https://piston-meta.mojang.com/mc/game/version_manifest_v2.json")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var manifest vanillaVersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// GetVersionStrings lists the Minecraft versions of a kind, newest first.
func GetVersionStrings(version_type string) ([]string, error) {
	manifest, err := GetMinecraftVersionsManifest()
	if err != nil {
		return nil, err
	}

	var versionList []string
	for _, v := range manifest.Versions {
		if v.Type == version_type {
			// v.ID is the string like "1.20.1" or "rd-132211"
			versionList = append(versionList, v.ID)
		}
	}

	return versionList, nil
}

func getVanillaServerURL(targetVersion string) (string, error) {
	// Step 1: Get the Master Manifest
	resp, err := getWithStatus("https://launchermeta.mojang.com/mc/game/version_manifest.json")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp, err = getWithStatus(versionDetailURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var pkg vanillaVersionPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", err
	}

	return pkg.Downloads.Server.URL, nil
}

// getPaperServerURL finds the newest Paper build for a Minecraft version.
//
// It asks PaperMC's fill API, which replaced the api.papermc.io/v2 endpoints
// the previous URLs were built from; those now answer 410 Gone to everything,
// so no Paper server could be downloaded at all.
func getPaperServerURL(version string) (string, error) {
	buildsURL := fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds", version)
	resp, err := getWithStatus(buildsURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var builds []paperBuild
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return "", err
	}

	if len(builds) == 0 {
		return "", fmt.Errorf("no builds found for version %s", version)
	}

	// The list is newest first, and a stable build is preferred over an
	// experimental one for a server someone is about to run.
	chosen := builds[0]
	for _, build := range builds {
		if strings.EqualFold(build.Channel, "STABLE") {
			chosen = build
			break
		}
	}

	download, ok := chosen.Downloads["server:default"]
	if !ok || download.URL == "" {
		return "", fmt.Errorf("paper build %d for %s has no server download", chosen.ID, version)
	}

	return download.URL, nil
}

func getFabricServerURL(mcVersion string) (string, error) {
	// Step 1: Get the latest stable loader version
	loaderResp, err := getWithStatus("https://meta.fabricmc.net/v2/versions/loader")
	if err != nil {
		return "", err
	}
	defer func() { _ = loaderResp.Body.Close() }()

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
	installerResp, err := getWithStatus("https://meta.fabricmc.net/v2/versions/installer")
	if err != nil {
		return "", err
	}
	defer func() { _ = installerResp.Body.Close() }()

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

func getNeoForgeVersionMap() map[string]string {
	resp, err := getWithStatus("https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml")
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	var metadata neoForgeVersion
	if err := xml.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil
	}

	versions, err := GetVersionStrings("release")
	if err != nil {
		return nil
	}

	versionMap := make(map[string]string)

	// matching
	for _, mcVersion := range versions {
		// Convert "1.21.1" -> "21.1" (The NeoForge Prefix)
		prefix := strings.TrimPrefix(mcVersion, "1.")

		// Find the latest NeoForge version that starts with that prefix
		var bestMatch string
		for _, neoforgeVersion := range metadata.Versions {
			if strings.HasPrefix(neoforgeVersion, prefix) {
				// Since Maven lists versions chronologically,
				// the last one we find for this prefix is the newest.
				bestMatch = neoforgeVersion
			}
		}

		if bestMatch != "" {
			versionMap[mcVersion] = bestMatch
		}
	}
	return versionMap
}

// getNeoForgeServerURL returns the installer JAR URL for NeoForge
func getNeoForgeServerURL(mcVersion string) (url, installer string, err error) {
	stableVersions := getNeoForgeVersionMap()

	neoforgeVersion, ok := stableVersions[mcVersion]
	if !ok {
		return "", "", fmt.Errorf("no stable NeoForge version configured for Minecraft %s", mcVersion)
	}

	// Construct installer URL
	// Format: https://maven.neoforged.net/releases/net/neoforged/neoforge/{version}/neoforge-{version}-installer.jar
	url = fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar",
		neoforgeVersion, neoforgeVersion)

	return url, neoforgeVersion, nil
}

func getForgeVersionMap() map[string]string {
	resp, err := getWithStatus("https://maven.minecraftforge.net/net/minecraftforge/forge/maven-metadata.xml")
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	var metadata forgeVersion
	if err := xml.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil
	}

	versions, err := GetVersionStrings("release")
	if err != nil {
		return nil
	}

	versionMap := make(map[string]string)

	// matching
	for _, mcVersion := range versions {
		// Find the latest NeoForge version that starts with that prefix
		var bestMatch string
		for _, neoforgeVersion := range metadata.Versions {
			if strings.HasPrefix(neoforgeVersion, mcVersion) {
				// Since Maven lists versions chronologically,
				// the last one we find for this prefix is the newest.
				bestMatch = neoforgeVersion
			}
		}

		if bestMatch != "" {
			versionMap[mcVersion] = bestMatch
		}
	}
	return versionMap
}

// getForgeServerURL returns the installer JAR URL for Forge
func getForgeServerURL(mcVersion string) (url, installer string, err error) {
	forgeVersionMap := getForgeVersionMap()
	forgeVersion, ok := forgeVersionMap[mcVersion]
	if !ok {
		return "", "", fmt.Errorf("no forge version for minecraft %s", mcVersion)
	}

	// Construct installer URL
	// Format: https://maven.minecraftforge.net/net/minecraftforge/forge/{forgeVer}/forge-{forgeVer}-installer.jar
	url = fmt.Sprintf("https://maven.minecraftforge.net/net/minecraftforge/forge/%s/forge-%s-installer.jar",
		forgeVersion, forgeVersion)

	return url, forgeVersion, nil
}

// getQuiltInstallerURL returns the Quilt installer JAR URL
func getQuiltInstallerURL() (url, version string, err error) {
	// Get the latest installer version from Quilt meta API
	resp, err := getWithStatus("https://meta.quiltmc.org/v3/versions/installer")
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var installers []quiltInstallerVersion
	if err := json.NewDecoder(resp.Body).Decode(&installers); err != nil {
		return "", "", err
	}

	if len(installers) == 0 {
		return "", "", fmt.Errorf("no Quilt installer versions found")
	}

	// Use the first (latest) installer
	installerVersion := installers[0].Version
	url = fmt.Sprintf("https://maven.quiltmc.org/repository/release/org/quiltmc/quilt-installer/%s/quilt-installer-%s.jar",
		installerVersion, installerVersion)

	return url, installerVersion, nil
}

// // getFabricAPIURL fetches the latest Fabric API version for a specific Minecraft version from Modrinth
// func getFabricAPIURL(mcVersion string) (string, string, error) {
// 	// Create HTTP client with proper User-Agent (required by Modrinth API)
// 	client := &http.Client{}
// 	req, err := http.NewRequest("GET", "https://api.modrinth.com/v2/project/fabric-api/version", nil)
// 	if err != nil {
// 		return "", "", err
// 	}
// 	req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return "", "", err
// 	}
// 	defer func() { _ = resp.Body.Close() }()

// 	if resp.StatusCode != http.StatusOK {
// 		return "", "", fmt.Errorf("Modrinth API error: %s", resp.Status)
// 	}

// 	var versions []modrinthVersion
// 	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
// 		return "", "", err
// 	}

// 	// Find the first release version that matches our MC version and is for fabric loader
// 	for _, v := range versions {
// 		// Check if this version supports our MC version
// 		supportsVersion := false
// 		for _, gv := range v.GameVersions {
// 			if gv == mcVersion {
// 				supportsVersion = true
// 				break
// 			}
// 		}

// 		if !supportsVersion {
// 			continue
// 		}

// 		// Check if it's for fabric loader
// 		supportsFabric := false
// 		for _, loader := range v.Loaders {
// 			if loader == "fabric" {
// 				supportsFabric = true
// 				break
// 			}
// 		}

// 		if !supportsFabric {
// 			continue
// 		}

// 		// Prefer release versions, but accept beta if no release found
// 		if v.VersionType == "release" || v.VersionType == "beta" {
// 			// Find the primary file
// 			for _, file := range v.Files {
// 				if file.Primary {
// 					return file.URL, file.Filename, nil
// 				}
// 			}
// 			// If no primary file, use the first one
// 			if len(v.Files) > 0 {
// 				return v.Files[0].URL, v.Files[0].Filename, nil
// 			}
// 		}
// 	}

// 	return "", "", fmt.Errorf("no Fabric API version found for Minecraft %s", mcVersion)
// }

// // DownloadFabricAPI downloads the Fabric API mod to the mods folder
// func DownloadFabricAPI(mcVersion string) error {
// 	// Create mods folder if it doesn't exist
// 	if err := os.MkdirAll("mods", 0o750); err != nil {
// 		return fmt.Errorf("failed to create mods folder: %w", err)
// 	}

// 	// Get Fabric API URL
// 	url, filename, err := getFabricAPIURL(mcVersion)
// 	if err != nil {
// 		return err
// 	}

// 	fmt.Println("Downloading Fabric API...")

// 	// Download with proper User-Agent
// 	client := &http.Client{}
// 	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
// 	if err != nil {
// 		return err
// 	}
// 	req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer func() { _ = resp.Body.Close() }()

// 	if resp.StatusCode != http.StatusOK {
// 		return fmt.Errorf("download failed: %s", resp.Status)
// 	}

// 	// Create the file in mods folder
// 	filepath := "mods/" + filename
// 	out, err := os.Create(filepath)
// 	if err != nil {
// 		return err
// 	}
// 	defer func() { _ = out.Close() }()

// 	_, err = io.Copy(out, resp.Body)
// 	if err != nil {
// 		return err
// 	}

// 	fmt.Printf("Fabric API downloaded: %s\n", filepath)
// 	return nil
// }

// progressWriter counts bytes on their way to the file and reports them, which
// is what turns a download into a progress bar rather than a wait.
type progressWriter struct {
	scope scope.Scope
	label string
	total int64
	done  int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	w.scope.Report(w.label, w.done, w.total)
	return len(p), nil
}

func downloadFile(sc scope.Scope, url, target, label string) error {
	// Create the output file
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	// Create a custom HTTP client with NO timeout (0 means no timeout limit)
	// This prevents the "context deadline exceeded" error on large files
	client := &http.Client{
		Timeout: 0 * time.Second,
	}

	// Create the request
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	// Add a generic user agent just in case APIs block default Go clients
	req.Header.Set("User-Agent", "Osmium-Server-Manager/1.0")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for a valid response code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to download file: bad status %s (url: %s)", resp.Status, url)
	}

	// Stream the download into the file, counting it on the way past.
	counter := &progressWriter{scope: sc, label: label, total: resp.ContentLength}
	sc.Report(label, 0, resp.ContentLength)

	if _, err = io.Copy(io.MultiWriter(out, counter), resp.Body); err != nil {
		return err
	}

	sc.Report(label, counter.total, counter.total)
	return nil
}

// DownloadJar downloads (and, for mod loaders, installs) the server files into
// the process working directory, printing progress to stdout. It is the form
// the Cobra commands use.
func DownloadJar(jarType, jarVersion string) error {
	return DownloadJarInto(scope.For("."), jarType, jarVersion)
}

// DownloadJarInto downloads the server files into the scope's directory and
// reports progress through the scope, so the TUI can show a build log instead
// of writing over the screen.
func DownloadJarInto(sc scope.Scope, jarType, jarVersion string) error {
	// Deciding which url
	url := ""
	var err error
	installerFilename := ""
	isInstaller := false

	// The engine is named by whoever wrote it down: a wizard writes "Fabric",
	// a modpack's index says "fabric-loader", and a manifest edited by hand
	// says whatever its author typed. Matching on the canonical spelling means
	// none of those has to be right.
	jarType = CanonicalLoader(jarType)

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
		url, neoforgeVer, err = getNeoForgeServerURL(jarVersion)
		if err != nil {
			return err
		}
		installerFilename = fmt.Sprintf("neoforge-%s-installer.jar", neoforgeVer)
		isInstaller = true
	case "Forge":
		var forgeVer string
		url, forgeVer, err = getForgeServerURL(jarVersion)
		if err != nil {
			return err
		}
		installerFilename = fmt.Sprintf("forge-%s-%s-installer.jar", jarVersion, forgeVer)
		isInstaller = true
	case "Quilt":
		var quiltInstallerVer string
		url, quiltInstallerVer, err = getQuiltInstallerURL()
		if err != nil {
			return err
		}
		installerFilename = fmt.Sprintf("quilt-installer-%s.jar", quiltInstallerVer)
		isInstaller = true
	}

	// Determine output filename
	output := "server.jar"
	if isInstaller {
		output = installerFilename
	}

	sc.Logf("Downloading the required files...")

	if err := downloadFile(sc, url, sc.Path(output), output); err != nil {
		return err
	}

	sc.Logf("Download finished: %s", output)

	// If it's an installer (NeoForge, Forge, Quilt), run the installer
	if isInstaller {
		sc.Logf("Running installer...")
		var err error
		if jarType == "Quilt" {
			err = RunModLoaderInstaller(sc, jarType, output, jarVersion)
		} else {
			err = RunModLoaderInstaller(sc, jarType, output)
		}
		if err != nil {
			return err
		}
	}

	//* This logic will be implemented in mod management
	// // For Fabric, automatically download Fabric API as it's required by most mods
	// if jarType == "Fabric" {
	// 	fmt.Println("\nFabric API is required by most Fabric mods. Downloading automatically...")
	// 	if err := DownloadFabricAPI(jarVersion); err != nil {
	// 		fmt.Printf("Warning: Could not download Fabric API: %v\n", err)
	// 		fmt.Println("You may need to download it manually from https://modrinth.com/mod/fabric-api")
	// 	}
	// }

	return nil
}

// RunModLoaderInstaller runs the mod loader installer to set up the server (for NeoForge, Forge, Quilt)
func RunModLoaderInstaller(sc scope.Scope, loaderType, installerFile string, mcVersion ...string) error {
	var cmd *exec.Cmd

	switch CanonicalLoader(loaderType) {
	case "NeoForge":
		// java -jar neoforge-installer.jar --installServer
		cmd = exec.Command("java", "-jar", installerFile, "--installServer")
	case "Forge":
		// java -jar forge-installer.jar --installServer
		cmd = exec.Command("java", "-jar", installerFile, "--installServer")
	case "Quilt":
		// java -jar quilt-installer.jar install server {mcVersion} --download-server
		if len(mcVersion) == 0 {
			return errors.New("quilt needs a minecraft version")
		}
		cmd = exec.Command("java", "-jar", installerFile, "install", "server", mcVersion[0], "--download-server")
	default:
		return fmt.Errorf("unknown mod loader type: %s", loaderType)
	}

	// Run inside the server directory, capturing the installer's chatter so it
	// can be shown in a log panel instead of the raw terminal.
	cmd.Dir = sc.Path()

	var installerOutput strings.Builder
	cmd.Stdout = &installerOutput
	cmd.Stderr = &installerOutput

	sc.Logf("Installing %s server...", loaderType)
	if err := cmd.Run(); err != nil {
		for _, line := range strings.Split(strings.TrimSpace(installerOutput.String()), "\n") {
			sc.Logf("%s", line)
		}
		return fmt.Errorf("installer failed: %w", err)
	}

	sc.Logf("Installation complete!")
	return nil
}

// Unnecessarily complicated function to append some text to a file
func appendToFile(path, line string) error {
	// Open the file in Append mode, Create it if it doesn't exist, and open as
	// Write-Only. 0o600: readable and writable by the user who runs Osmium.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Write the string to the file
	_, err = file.WriteString(line)
	if err != nil {
		return err
	}

	return nil
}

// GetServerRunCommand returns the command needed to run the server for a given
// jar type. dir is the server directory; it is where the loader-specific JVM
// argument file is written.
func GetServerRunCommand(dir, jarType, ram string) (command string, args []string) {
	switch CanonicalLoader(jarType) {
	case "NeoForge", "Forge":
		// Since NeoForge & Forge both accept arguments only through writing to a specific text file, here it is.
		// A heap size the loader cannot be told any other way. If the file
		// cannot be written the server still starts, on its own defaults.
		_ = appendToFile(filepath.Join(dir, "user_jvm_args.txt"), "\n-Xms"+ram+"\n-Xmx"+ram+"\n")
		// NeoForge and Forge create run.bat/run.sh after installation - use those directly
		switch runtime.GOOS {
		case "windows": // On Windows specifically, run the batch file
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
	case "Quilt":
		// Quilt creates quilt-server-launch.jar after installation
		return "java", []string{"-jar", "-Xms" + ram, "-Xmx" + ram, "./server/server.jar", "nogui"}
	default:
		// Standard server.jar execution
		return "java", []string{"-jar", "-Xms" + ram, "-Xmx" + ram, "server.jar", "nogui"}
	}
}

// Loaders are the engines Osmium can install, spelled the way this package
// matches them.
var Loaders = []string{"Vanilla", "Paper", "Purpur", "Fabric", "Quilt", "Forge", "NeoForge"}

// CanonicalLoader is an engine's name as this package spells it, whatever case
// it was given in.
//
// The name travels through a wizard, a manifest, a modpack's index and a
// command-line flag before it gets here, and each of those has its own idea of
// capitalisation. Matching a switch against one spelling and silently doing
// nothing for the others is how "unsupported protocol scheme" ends up in front
// of somebody who typed --loader fabric.
func CanonicalLoader(name string) string {
	for _, known := range Loaders {
		if strings.EqualFold(known, strings.TrimSpace(name)) {
			return known
		}
	}
	return strings.TrimSpace(name)
}
