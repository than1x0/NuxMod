package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/daemon"
)

const (
	CurrentVersion = "2.0.0"
	GitHubRepo     = "Sathira-Laksahan/vless-linux"
	InstallPath    = "/usr/local/bin/nuxmod"
	SymlinkPath    = "/usr/local/bin/vless-linux"
)

// UpdateInfo holds release information from GitHub.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
	DownloadURL    string
	ReleaseNotes   string
}

// GitHubRelease represents GitHub Releases API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdate queries GitHub for the latest release tag.
func CheckForUpdate() (*UpdateInfo, error) {
	info := &UpdateInfo{
		CurrentVersion: CurrentVersion,
		LatestVersion:  CurrentVersion,
		HasUpdate:      false,
		DownloadURL:    fmt.Sprintf("https://github.com/%s/releases/latest/download/nuxmod", GitHubRepo),
	}

	client := &http.Client{Timeout: 5 * time.Second}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", "NuxMod-Updater/"+CurrentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("network error checking updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var rel GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&rel); err == nil {
			latest := strings.TrimPrefix(rel.TagName, "v")
			info.LatestVersion = latest
			info.ReleaseNotes = rel.Body

			targetAsset := "nuxmod"
			if runtime.GOARCH == "arm64" {
				targetAsset = "nuxmod-arm64"
			}
			for _, a := range rel.Assets {
				if a.Name == targetAsset || a.Name == "nuxmod" {
					info.DownloadURL = a.BrowserDownloadURL
					break
				}
			}

			if isNewerVersion(CurrentVersion, latest) {
				info.HasUpdate = true
			}
		}
	} else if resp.StatusCode == http.StatusNotFound {
		// Fallback: Check raw repository commit/version
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/cmd/nuxmod/main.go", GitHubRepo)
		if rawResp, err := client.Get(rawURL); err == nil && rawResp.StatusCode == http.StatusOK {
			defer rawResp.Body.Close()
			body, _ := io.ReadAll(rawResp.Body)
			if strings.Contains(string(body), "AppVersion") {
				// Repo reachable
				info.DownloadURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/nuxmod", GitHubRepo)
			}
		}
	}

	return info, nil
}

// DownloadAndInstall downloads the latest binary and installs it to /usr/local/bin/nuxmod.
func DownloadAndInstall(downloadURL string) error {
	if downloadURL == "" {
		downloadURL = fmt.Sprintf("https://github.com/%s/releases/latest/download/nuxmod", GitHubRepo)
	}

	tmpFile := fmt.Sprintf("/tmp/nuxmod-update-%d", time.Now().Unix())
	defer os.Remove(tmpFile)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return fmt.Errorf("failed to save download: %w", err)
	}
	out.Close()

	// Verify the downloaded file is not empty
	stat, err := os.Stat(tmpFile)
	if err != nil || stat.Size() < 100000 {
		return fmt.Errorf("downloaded binary appears corrupt or incomplete (size: %d bytes)", stat.Size())
	}

	// Was daemon running?
	wasRunning := daemon.IsDaemonRunning()
	if wasRunning {
		_ = daemon.StopBackgroundDaemon()
		time.Sleep(500 * time.Millisecond)
	}

	// In-place replace using install (unlinks inode before write, avoiding Text file busy)
	cmd := exec.Command("install", "-m", "755", tmpFile, InstallPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install failed: %s (%w)", string(out), err)
	}

	// Update backwards compatibility symlink
	_ = exec.Command("ln", "-sf", InstallPath, SymlinkPath).Run()

	// Set network capabilities
	_ = exec.Command("setcap", "cap_net_admin,cap_net_bind_service+ep", InstallPath).Run()

	// If daemon was running, restart with the new binary
	if wasRunning {
		_ = daemon.StartBackgroundDaemon()
	}

	return nil
}

func isNewerVersion(current, latest string) bool {
	cParts := strings.Split(current, ".")
	lParts := strings.Split(latest, ".")
	for i := 0; i < len(cParts) && i < len(lParts); i++ {
		cV, err1 := strconv.Atoi(cParts[i])
		lV, err2 := strconv.Atoi(lParts[i])
		if err1 == nil && err2 == nil {
			if lV > cV {
				return true
			} else if lV < cV {
				return false
			}
		} else {
			if lParts[i] > cParts[i] {
				return true
			} else if lParts[i] < cParts[i] {
				return false
			}
		}
	}
	return len(lParts) > len(cParts)
}
