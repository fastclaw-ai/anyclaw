package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/version"
	"github.com/spf13/cobra"
)

const (
	repoOwner = "fastclaw-ai"
	repoName  = "anyclaw"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update anyclaw to the latest version",
	RunE:  runUpdate,
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Update anyclaw to the latest version (alias for update)",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(upgradeCmd)
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking for updates...")

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check latest release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version.Version, "v")

	if current == latest {
		fmt.Printf("Already up to date: %s\n", version.Version)
		return nil
	}

	fmt.Printf("Current: %s -> Latest: %s\n", version.Version, release.TagName)

	// Find the right asset for this OS/arch
	assetName := fmt.Sprintf("anyclaw_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s, you can build from source:\n  go install github.com/fastclaw-ai/anyclaw@latest", runtime.GOOS, runtime.GOARCH)
	}

	// Download
	fmt.Printf("Downloading %s...\n", assetName)
	tmpDir, err := os.MkdirTemp("", "anyclaw-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(tarPath, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract
	extractCmd := exec.Command("tar", "xzf", tarPath, "-C", tmpDir)
	if out, err := extractCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract failed: %s %w", string(out), err)
	}

	newBinary := filepath.Join(tmpDir, "anyclaw")
	if _, err := os.Stat(newBinary); err != nil {
		return fmt.Errorf("extracted binary not found: %w", err)
	}

	// Replace current binary
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current binary path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	if err := replaceBinary(selfPath, newBinary); err != nil {
		return fmt.Errorf("failed to replace binary: %w\nTry: sudo anyclaw update", err)
	}

	fmt.Printf("Updated to %s\n", release.TagName)
	return nil
}

func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func replaceBinary(dst, src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Write to a temp file next to the target, then rename (atomic on most FS)
	tmpPath := dst + ".new"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, srcFile); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	out.Close()

	return os.Rename(tmpPath, dst)
}
